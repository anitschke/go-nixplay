package nixplay

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net/http"
	"strconv"

	"github.com/anitschke/go-nixplay/httpx"
	"github.com/anitschke/go-nixplay/internal/cache"
	"github.com/anitschke/go-nixplay/internal/errorx"
	"github.com/anitschke/go-nixplay/types"
)

const photoPageSize = uint64(100)

// xxx doc assume first page is page 0
type photoPageFunc = func(ctx context.Context, client httpx.Client, container Container, nixplayID uint64, page uint64, pageSize uint64) ([]Photo, error)
type deleteRequestFunc = func(ctx context.Context, nixplayID uint64) (*http.Request, error)

type container struct {
	containerType types.ContainerType
	name          string
	id            types.ID
	photoCount    int64 //xxx add test for photo count

	client    httpx.Client
	nixplayID uint64

	photoCache *cache.Cache[Photo]

	photoPageFunc     photoPageFunc
	deleteRequestFunc deleteRequestFunc
	addIDName         string
}

func newContainer(client httpx.Client, containerType types.ContainerType, name string, nixplayID uint64, photoCount int64, photoPageFunc photoPageFunc, deleteRequestFunc deleteRequestFunc, addIDName string) *container {

	nixplayIdAsBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nixplayIdAsBytes, nixplayID)
	hasher := sha256.New()
	hasher.Write([]byte(containerType))
	hasher.Write(nixplayIdAsBytes)
	id := types.ID(hasher.Sum([]byte{}))

	c := &container{
		containerType:     containerType,
		client:            client,
		name:              name,
		id:                id,
		nixplayID:         nixplayID,
		photoCount:        photoCount,
		photoPageFunc:     photoPageFunc,
		deleteRequestFunc: deleteRequestFunc,
		addIDName:         addIDName,
	}

	c.photoCache = cache.NewCache(c.photosPage)

	return c
}

var _ = (Container)((*container)(nil))

func (c *container) ContainerType() types.ContainerType {
	return c.containerType
}

func (c *container) Name(ctx context.Context) (string, error) {
	// While we don't need the context and won't ever produce an error we will
	// still use this API so it has a consistent interface as Photo.Name().
	return c.name, nil
}

func (c *container) ID() types.ID {
	return c.id
}

func (c *container) PhotoCount(ctx context.Context) (retCount int64, err error) {
	return c.photoCount, nil
}

func (c *container) Delete(ctx context.Context) (err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	req, err := c.deleteRequestFunc(ctx, c.nixplayID)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	return httpx.StatusError(resp)
}

func (c *container) Photos(ctx context.Context) (retPhotos []Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return c.photoCache.All(ctx)
}

func (c *container) PhotosWithName(ctx context.Context, name string) (retPhoto []Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return c.photoCache.PhotosWithName(ctx, name)
}

func (c *container) PhotoWithID(ctx context.Context, id types.ID) (retPhoto Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return c.photoCache.PhotoWithID(ctx, id)
}

func (c *container) photosPage(ctx context.Context, page uint64) ([]Photo, error) {
	return c.photoPageFunc(ctx, c.client, c, c.nixplayID, page, photoPageSize)
}

func (c *container) AddPhoto(ctx context.Context, name string, r io.Reader, opts AddPhotoOptions) (retPhoto Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)

	albumID := uploadContainerID{
		idName: c.addIDName,
		id:     strconv.FormatUint(c.nixplayID, 10),
	}

	photoData, err := addPhoto(ctx, c.client, albumID, name, r, opts)
	if err != nil {
		return nil, err
	}

	nixplayPhotoID := uint64(0)
	photoURL := ""
	p, err := newPhoto(c, c.client, name, &photoData.md5Hash, nixplayPhotoID, photoData.size, photoURL)
	c.photoCache.Add(p)
	return p, err
}

func (c *container) ResetCache() {
	c.photoCache.Reset()
}
