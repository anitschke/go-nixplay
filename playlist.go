package nixplay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/anitschke/go-nixplay/httpx"
	"github.com/anitschke/go-nixplay/internal/cache"
	"github.com/anitschke/go-nixplay/internal/errorx"
	"github.com/anitschke/go-nixplay/types"
)

//xxx all the data getting stored is the same and almost all the methods are the
//same, so I need to look into making common container type I can use here.

type playlist struct {
	name       string
	id         types.ID
	photoCount int64

	client    httpx.Client
	nixplayID uint64

	photoCache *cache.Cache[Photo]
}

func newPlaylist(client httpx.Client, name string, nixplayID uint64, photoCount int64) *playlist {
	var id types.ID
	binary.LittleEndian.PutUint64(id[:], nixplayID)
	id = sha256.Sum256(id[:])

	p := &playlist{
		client:     client,
		name:       name,
		id:         id,
		nixplayID:  nixplayID,
		photoCount: photoCount,
	}

	p.photoCache = cache.NewCache(p.playlistPhotosPage)

	return p
}

var _ = (Container)((*playlist)(nil))

func (p *playlist) ContainerType() types.ContainerType {
	return types.PlaylistContainerType
}

func (p *playlist) Name() string {
	return p.name
}

func (p *playlist) ID() types.ID {
	return p.id
}

func (p *playlist) PhotoCount(ctx context.Context) (int64, error) {
	return p.photoCount, nil
}

func (p *playlist) Delete(ctx context.Context) (err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	url := fmt.Sprintf("https://api.nixplay.com/v3/playlists/%d", p.nixplayID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	if err = httpx.StatusError(resp); err != nil {
		return err
	}
	return nil
}

func (p *playlist) Photos(ctx context.Context) (retPhotos []Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err) //xxx ohh I think a lot of these need defers
	return p.photoCache.All(ctx)
}

func (p *playlist) PhotosWithName(ctx context.Context, name string) (retPhotos []Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return p.photoCache.PhotosWithName(ctx, name)
}

func (p *playlist) PhotoWithID(ctx context.Context, id types.ID) (retPhoto Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return p.photoCache.PhotoWithID(ctx, id)
}

// xxx I think we can leave the size an offset off to just get all the photos in
// one page. This simplifies things a lot. before you make this change confirm
// it will work by adding a test that adds 1000 photos (this is more than
// default size for either album or playlist)
func (p *playlist) playlistPhotosPage(ctx context.Context, page uint64) ([]Photo, error) {
	limit := uint64(photoPageSize) //same limit used by nixplay.com when getting photos
	offset := page * limit
	url := fmt.Sprintf("https://api.nixplay.com/v3/playlists/%d/slides?size=%d&offset=%d", p.nixplayID, limit, offset)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, bytes.NewReader([]byte{}))
	if err != nil {
		return nil, err
	}

	var playlistPhotos playlistPhotosResponse
	if err := httpx.DoUnmarshalJSONResponse(p.client, req, &playlistPhotos); err != nil {
		return nil, err
	}

	return playlistPhotos.ToPhotos(p, p.client)
}

func (p *playlist) AddPhoto(ctx context.Context, name string, r io.Reader, opts AddPhotoOptions) (retPhoto Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	albumID := uploadContainerID{
		idName: "playlistId",
		id:     strconv.FormatUint(p.nixplayID, 10),
	}

	photoData, err := addPhoto(ctx, p.client, albumID, name, r, opts)
	if err != nil {
		return nil, err
	}

	nixplayPhotoID := uint64(0)
	photoURL := ""

	photo, err := newPhoto(p, p.client, name, &photoData.md5Hash, nixplayPhotoID, photoData.size, photoURL)
	p.photoCache.Add(photo)
	return photo, err
}

func (p *playlist) ResetCache() {
	p.photoCache.Reset()
}

func (p *playlist) onPhotoDelete(ctx context.Context, photo Photo) error {
	return p.photoCache.Remove(ctx, photo)
}
