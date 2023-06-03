package nixplay

import (
	"context"
	"sync"

	"github.com/anitschke/go-nixplay/types"
)

//xxx can this be made a generic cache for storing both photos and containers?
//perhaps with generics?

// PhotoPageFunc is a function that when provided a page number can provide all
// photos on that page.
//
// Page number starts at 0
type photoPageFunc = func(ctx context.Context, page uint64) ([]Photo, error)

// photoCache provides caching of the photos within a container so we do not
// need to do a lookup every time we want info on a photo of a specific name.
type photoCache struct {
	photoPageFunc photoPageFunc

	mu           sync.Mutex
	nextPage     uint64
	foundAll     bool
	photos       []Photo
	nameToPhotos map[string][]Photo
	idToPhoto    map[types.ID]Photo
}

func newPhotoCache(photoPageFunc photoPageFunc) *photoCache {
	return &photoCache{
		photoPageFunc: photoPageFunc,
		nameToPhotos:  nil,
		idToPhoto:     make(map[types.ID]Photo),
	}
}

//xxx add tests, could deadlock with all this mutex use

//xxx add a panic/assert to all unsafe methods that mutex is already locked, at least for now

//xxx add ability to turn off caching for testing

//xxx add ability for external code to add/remove photos from cache so we can
//handle add and remove of photos

//

// All will return all photos
//
// If all photos for this container are already in the cache then it will return
// directly from the cache. If not all photos are known then it will build the
// cache by asking for pages until it discovers a page that has no photos and
// then returns all photos in the cache.
func (pc *photoCache) All(ctx context.Context) ([]Photo, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// xxx simplify all this because now I don't have a walk function (because it might deadlock)

	if err := pc.loadAllUnsafe(ctx); err != nil {
		return nil, err
	}

	return pc.photos, nil //xxx in theory we should make a copy of this slice so it can't get modified on the outside
}

// get photos with a specific name. In the event that there are no photos with
// the specified name nil is returned
func (pc *photoCache) PhotosWithName(ctx context.Context, name string) ([]Photo, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if err := pc.loadAllUnsafe(ctx); err != nil {
		return nil, err
	}

	if err := pc.populateNameMapUnsafe(ctx); err != nil {
		return nil, err
	}

	photos := pc.nameToPhotos[name]
	return photos, nil
}

// get the photo with the specified ID. In the event that there is no photo with the specified ID
// a nil Photo is returned
func (pc *photoCache) PhotoWithID(ctx context.Context, id types.ID) (Photo, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if err := pc.loadAllUnsafe(ctx); err != nil {
		return nil, err
	}

	return pc.idToPhoto[id], nil
}

// Load all photos into the cache. It assumes the mutex guarding the
// cache is already locked.
func (pc *photoCache) loadAllUnsafe(ctx context.Context) (err error) {
	for !pc.foundAll {
		_, err := pc.loadNextPageUnsafe(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// loads the next page into the cache. It assumes the mutex guarding the cache
// is already locked. Any new photos loaded as part of the next page will be
// returned
func (pc *photoCache) loadNextPageUnsafe(ctx context.Context) ([]Photo, error) {
	// xxx I think we can leave the size an offset off to just get all the photos in
	// one page. This simplifies things a lot. before you make this change confirm
	// it will work by adding a test that adds 1000 photos (this is more than
	// default size for either album or playlist)

	photos, err := pc.photoPageFunc(ctx, pc.nextPage)
	if err != nil {
		return nil, err
	}
	if len(photos) == 0 {
		pc.foundAll = true
	}
	for _, p := range photos {
		pc.addPhotoUnsafe(p)
	}

	pc.nextPage++

	return photos, nil
}

// Add may be called to add a photo to the cache. This can be useful when a
// photo is created
func (pc *photoCache) Add(p Photo) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.addPhotoUnsafe(p)
}

// addPhotoUnsafe adds a photo to the cache. It assumes the mutex guarding the
// cache is already locked.
//
// The nameToPhotos map is not populated as part of this because sometimes
// getting the name of a photo requires a network call (for playlists that were
// not uploaded) In addition as soon as a new photo is added to the cache the
// nameToPhotos map is no longer valid because we may not have a name for that
// photo yet. So we reset the nameToPhotos when adding a new photo to the cache.
func (pc *photoCache) addPhotoUnsafe(p Photo) {

	// If the photo is already in the cache  just early return
	if _, ok := pc.idToPhoto[p.ID()]; ok {
		return
	}

	pc.photos = append(pc.photos, p)

	id := p.ID()
	pc.idToPhoto[id] = p

	pc.nameToPhotos = nil
}

func (pc *photoCache) populateNameMapUnsafe(ctx context.Context) (err error) {
	if pc.nameToPhotos != nil {
		return nil
	}

	defer func() {
		if err != nil {
			pc.nameToPhotos = nil
		}
	}()

	pc.nameToPhotos = make(map[string][]Photo)
	for _, p := range pc.photos {
		name, err := p.Name(ctx)
		if err != nil {
			return err
		}
		pc.nameToPhotos[name] = append(pc.nameToPhotos[name], p)
	}
	return nil
}

func (pc *photoCache) Remove(ctx context.Context, p Photo) (err error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	defer func() {
		if err != nil {
			pc.resetUnsafe()
		}
	}()

	// If the photo isn't in the cache at all just early return
	cachedPhoto, ok := pc.idToPhoto[p.ID()]
	if !ok {
		return nil
	}

	// Delete photo from the pc.photos slice
	id := p.ID()
	for i, possible := range pc.photos {
		if id == possible.ID() {
			pc.photos[i] = pc.photos[len(pc.photos)-1]
			pc.photos = pc.photos[:len(pc.photos)-1]
			break
		}
	}

	// Delete the photo from the nameToPhotos map / slice
	if pc.nameToPhotos != nil {
		// The photo provided to Remove may not be the same photo object that we
		// have in memory in the cache. If we have the pc.nameToPhotos then we
		// know that the photo object that we have in the cache should know it's
		// name because it had to request it to populate the cache. So lets
		// lookup the photo that is in the cache since that should guarantee
		// that we know the name without needing to make a web request to get
		// it.

		name, err := cachedPhoto.Name(ctx)
		if err != nil {
			return err
		}

		s := pc.nameToPhotos[name]
		for i, possible := range s {
			if p.ID() == possible.ID() {
				if len(s) == 1 {
					delete(pc.nameToPhotos, name)
					break
				}
				s[i] = s[len(s)-1]
				s = s[:len(s)-1]
				pc.nameToPhotos[name] = s
				break
			}
		}
	}

	// Delete the photo from the idToPhoto map
	delete(pc.idToPhoto, p.ID())

	return nil
}

// Reset should be called in situations where the cache may no longer be valid
// any more to reset all cache state
func (pc *photoCache) Reset() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.resetUnsafe()
}

// resetUnsafe does the same as Reset but assumes that the mutex guarding the
// cache is already locked
func (pc *photoCache) resetUnsafe() {
	pc.nextPage = 0
	pc.foundAll = false
	pc.photos = nil
	pc.nameToPhotos = nil
	pc.idToPhoto = make(map[types.ID]Photo)
}
