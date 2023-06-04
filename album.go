package nixplay

import (
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

const photoPageSize = 100

type album struct {
	name       string
	id         types.ID
	photoCount int64

	client    httpx.Client
	nixplayID uint64

	photoCache *cache.Cache[Photo]
}

func newAlbum(client httpx.Client, name string, nixplayID uint64, photoCount int64) *album {
	var id types.ID
	binary.LittleEndian.PutUint64(id[:], nixplayID)
	id = sha256.Sum256(id[:])

	a := &album{
		client:     client,
		name:       name,
		id:         id,
		nixplayID:  nixplayID,
		photoCount: photoCount,
	}

	a.photoCache = cache.NewCache(a.albumPhotosPage)

	return a
}

var _ = (Container)((*album)(nil))

func (a *album) ContainerType() types.ContainerType {
	return types.AlbumContainerType
}

func (a *album) Name() string {
	return a.name
}

func (a *album) ID() types.ID {
	return a.id
}

func (a *album) PhotoCount(ctx context.Context) (retCount int64, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return a.photoCount, nil
}

func (a *album) Delete(ctx context.Context) (err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	url := fmt.Sprintf("https://api.nixplay.com/album/%d/delete/json/", a.nixplayID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	return httpx.StatusError(resp)
}

func (a *album) Photos(ctx context.Context) (retPhotos []Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return a.photoCache.All(ctx)
}

func (a *album) PhotosWithName(ctx context.Context, name string) (retPhoto []Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return a.photoCache.PhotosWithName(ctx, name)
}

func (a *album) PhotoWithID(ctx context.Context, id types.ID) (retPhoto Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return a.photoCache.PhotoWithID(ctx, id)
}

func (a *album) albumPhotosPage(ctx context.Context, page uint64) ([]Photo, error) {
	page++ // nixplay uses 1 based indexing for album pages but photoCache assumes 0 based.

	limit := photoPageSize //same limit used by nixplay.com when getting photos
	url := fmt.Sprintf("https://api.nixplay.com/album/%d/pictures/json/?page=%d&limit=%d", a.nixplayID, page, limit)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	var albumPhotos albumPhotosResponse
	if err := httpx.DoUnmarshalJSONResponse(a.client, req, &albumPhotos); err != nil {
		return nil, err
	}

	return albumPhotos.ToPhotos(a, a.client)
}

func (a *album) AddPhoto(ctx context.Context, name string, r io.Reader, opts AddPhotoOptions) (retPhoto Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

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
