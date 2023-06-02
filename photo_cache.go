package nixplay

import (
	"context"
	"sync"
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
	idToPhoto    map[ID]Photo
}

func newPhotoCache(photoPageFunc photoPageFunc) *photoCache {
	return &photoCache{
		photoPageFunc: photoPageFunc,
		nameToPhotos:  make(map[string][]Photo),
		idToPhoto:     make(map[ID]Photo),
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

	photos := pc.nameToPhotos[name]
	return photos, nil
}

// get the photo with the specified ID. In the event that there is no photo with the specified ID
// a nil Photo is returned
func (pc *photoCache) PhotoWithID(ctx context.Context, id ID) (Photo, error) {
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
func (pc *photoCache) addPhotoUnsafe(p Photo) {
	pc.photos = append(pc.photos, p)

	name := p.Name()
	pc.nameToPhotos[name] = append(pc.nameToPhotos[name], p)

	id := p.ID()
	pc.idToPhoto[id] = p
}

func (pc *photoCache) Remove(p Photo) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

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
	name := p.Name()
	s := pc.nameToPhotos[name]
	for i, possible := range s {
		if p == possible {
			s[i] = s[len(s)-1]
			pc.nameToPhotos[name] = s[:len(s)-1]
			break
		}
	}

	// Delete the photo from the idToPhoto map
	delete(pc.idToPhoto, p.ID())
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
	pc.nameToPhotos = make(map[string][]Photo)
	pc.idToPhoto = make(map[ID]Photo)
}
