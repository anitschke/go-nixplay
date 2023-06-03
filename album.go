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
)

type album struct {
	name       string
	id         ID
	photoCount int64

	client    httpx.Client
	nixplayID uint64

	photoCache *photoCache
}

func newAlbum(client httpx.Client, name string, nixplayID uint64, photoCount int64) *album {
	var id ID
	binary.LittleEndian.PutUint64(id[:], nixplayID)
	id = sha256.Sum256(id[:])

	a := &album{
		client:     client,
		name:       name,
		id:         id,
		nixplayID:  nixplayID,
		photoCount: photoCount,
	}

	a.photoCache = newPhotoCache(a.albumPhotosPage)

	return a
}

var _ = (Container)((*album)(nil))

func (a *album) ContainerType() ContainerType {
	return AlbumContainerType
}

func (a *album) Name() string {
	return a.name
}

func (a *album) ID() ID {
	return a.id
}

func (a *album) PhotoCount(ctx context.Context) (int64, error) {
	return a.photoCount, nil
}

func (a *album) Delete(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to delete album: %w", err)
		}
	}()

	url := fmt.Sprintf("https://api.nixplay.com/album/%d/delete/json/", a.nixplayID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body) //xxx track down any places where I may not be reading the response

	return httpx.StatusError(resp)
}

func (a *album) Photos(ctx context.Context) (retPhotos []Photo, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get photos: %w", err)
		}
	}()

	return a.photoCache.All(ctx)
}

func (a *album) PhotosWithName(ctx context.Context, name string) ([]Photo, error) {
	return a.photoCache.PhotosWithName(ctx, name)
}

func (a *album) PhotoWithID(ctx context.Context, id ID) (Photo, error) {
	return a.photoCache.PhotoWithID(ctx, id)
}

// xxx I think we can leave the size an offset off to just get all the photos in
// one page. This simplifies things a lot. before you make this change confirm
// it will work by adding a test that adds 1000 photos (this is more than
// default size for either album or playlist)
func (a *album) albumPhotosPage(ctx context.Context, page uint64) ([]Photo, error) {
	page++ // nixplay uses 1 based indexing for album pages but photoCache assumes 0 based.

	//xxx test multiple pages somehow
	limit := 500 //same limit used by nixplay.com when getting photos
	url := fmt.Sprintf("https://api.nixplay.com/album/%d/pictures/json/?page=%d&limit=%d", a.nixplayID, page, limit)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, bytes.NewReader([]byte{}))
	if err != nil {
		return nil, err
	}

	var albumPhotos albumPhotosResponse
	if err := httpx.DoUnmarshalJSONResponse(a.client, req, &albumPhotos); err != nil {
		return nil, err
	}

	return albumPhotos.ToPhotos(a, a.client)
}

func (a *album) AddPhoto(ctx context.Context, name string, r io.Reader, opts AddPhotoOptions) (Photo, error) {
	albumID := uploadContainerID{
		idName: "albumId",
		id:     strconv.FormatUint(a.nixplayID, 10),
	}

	photoData, err := addPhoto(ctx, a.client, albumID, name, r, opts)
	if err != nil {
		return nil, err
	}

	nixplayPhotoID := uint64(0)
	photoURL := ""
	p, err := newPhoto(a, a.client, name, &photoData.md5Hash, nixplayPhotoID, photoData.size, photoURL)
	a.photoCache.Add(p)
	return p, err
}

func (a *album) ResetCache() {
	a.photoCache.Reset()
}

func (a *album) onPhotoDelete(ctx context.Context, photo Photo) error {
	return a.photoCache.Remove(ctx, photo)
}
