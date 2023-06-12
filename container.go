package nixplay

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/anitschke/go-nixplay/httpx"
	"github.com/anitschke/go-nixplay/internal/cache"
	"github.com/anitschke/go-nixplay/internal/errorx"
	"github.com/anitschke/go-nixplay/types"
)

// photoPageSize is the number of photos we will request per album/playlist page
// of photos. In theory we might be able to simplify the code by getting all the
// photos in a single request but I am not sure if the API may automatically
// paginate at some point. So we will just play it on the safe side.
const photoPageSize = uint64(100)

// photoPageFunc is a function that returns the photos on a the specified page.
// The first page is page 0.
type photoPageFunc = func(ctx context.Context, client httpx.Client, container Container, nixplayID uint64, page uint64, pageSize uint64) ([]Photo, error)

// deleteRequestFunc is a function that can be used to create a *http.Request to
// delete a photo.
type deleteRequestFunc = func(ctx context.Context, nixplayID uint64) (*http.Request, error)

type container struct {
	containerType types.ContainerType
	name          string
	id            types.ID

	// photoCount can change over time so it must be guarded by a mutex
	photoCountMu sync.Mutex
	photoCount   int64

	client    httpx.Client
	nixplayID uint64

	photoCache             *cache.Cache[Photo]
	elementDeletedListener []cache.ElementDeletedListener

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
	id := *(*types.ID)(hasher.Sum([]byte{}))

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
	c.photoCache.AddDeletedListener(c)

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
	c.photoCountMu.Lock()
	defer c.photoCountMu.Unlock()

	if c.photoCount == -1 {
		count, err := c.photoCache.ElementCount(ctx)
		if err != nil {
			return 0, err
		}
		c.photoCount = count
	}

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

	if err := httpx.StatusError(resp); err != nil {
		return err
	}

	for _, l := range c.elementDeletedListener {
		if err := l.ElementDeleted(ctx, c); err != nil {
			return err
		}
	}

	return nil
}

func (c *container) AddDeletedListener(l cache.ElementDeletedListener) {
	c.elementDeletedListener = append(c.elementDeletedListener, l)
}

func (c *container) Photos(ctx context.Context) (retPhotos []Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return c.photoCache.All(ctx)
}

func (c *container) PhotosWithName(ctx context.Context, name string) (retPhoto []Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return c.photoCache.ElementsWithName(ctx, name)
}

func (c *container) PhotoWithUniqueName(ctx context.Context, name string) (retPhoto Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return c.photoCache.ElementWithUniqueName(ctx, name)
}

func (c *container) PhotoWithID(ctx context.Context, id types.ID) (retPhoto Photo, err error) {
	defer errorx.WrapWithFuncNameIfError(&err)
	return c.photoCache.ElementWithID(ctx, id)
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
	if errors.Is(err, errDuplicateImage) && c.containerType == types.PlaylistContainerType {
		// See https://github.com/anitschke/go-nixplay/#nixplay-meta-model
		//
		// Nixplay doesn't allow photos with duplicate content in the same
		// album. This can make uploading to playlists a little tricky as it
		// seems what nixplay really does is upload it directly to the "My
		// Uploads" album and then behind the scenes links the photo in the "My
		// Uploads" album with the playlist you tried to upload to. This gets
		// tricky because if you try to upload the same photo to multiple
		// playlists the upload monitor errors out indicating that there was a
		// duplicate, because there was a duplicate in the "My Uploads" album,
		// not necessarily because there was a duplicate in the playlist (which
		// is allowed anyway.) Even when the upload monitor errors out like this
		// the photo still gets added to the playlist so like we wanted.
		//
		// So long story short if we are uploading to a container and we get the
		// errDuplicateImage we can just ignore the error and continue like
		// normal.
		err = nil
	}
	if err != nil {
		return nil, err
	}

	nixplayPhotoID := uint64(0)
	nixplayPlaylistItemID := ""
	photoURL := ""
	p, err := newPhoto(c, c.client, name, &photoData.md5Hash, nixplayPhotoID, nixplayPlaylistItemID, photoData.size, photoURL)
	if err != nil {
		return nil, err
	}

	c.photoCache.Add(p)

	c.photoCountMu.Lock()
	defer c.photoCountMu.Unlock()
	c.photoCount++

	return p, nil
}

// Listens to deletes of photos from the cache
func (c *container) ElementDeleted(ctx context.Context, e cache.Element) (err error) {
	c.photoCountMu.Lock()
	defer c.photoCountMu.Unlock()
	c.photoCount--
	return nil
}

func (c *container) ResetCache() {
	c.photoCache.Reset()
}
