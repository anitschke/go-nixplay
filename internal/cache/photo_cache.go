package cache

import (
	"context"
	"sync"

	"github.com/anitschke/go-nixplay/types"
)

type Element interface {
	ID() types.ID
	Name(ctx context.Context) (string, error)
}

// elementPageFunc is a function that when provided a page number can provide
// all elements on that page.
//
// Page number starts at 0
type elementPageFunc[T Element] func(ctx context.Context, page uint64) ([]T, error)

// Cache provides caching of containers or photos within a container so we do
// not need to do a HTTP request to lookup info every time we want info on an
// element.
type Cache[T Element] struct {
	elementPageFunc elementPageFunc[T]

	mu             sync.Mutex
	foundAll       bool
	elements       []T
	nameToElements map[string][]T
	idToElement    map[types.ID]T
}

func NewCache[T Element](elementPageFunc elementPageFunc[T]) *Cache[T] {
	return &Cache[T]{
		elementPageFunc: elementPageFunc,
		nameToElements:  nil,
		idToElement:     make(map[types.ID]T),
	}
}

// All will return all elements
//
// If all elements for this container are already in the cache then it will return
// directly from the cache. If not all elements are known then it will build the
// cache by asking for pages until it discovers a page that has no elements and
// then returns all elements in the cache.
func (c *Cache[T]) All(ctx context.Context) ([]T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.loadAllUnsafe(ctx); err != nil {
		return nil, err
	}

	elements := make([]T, len(c.elements))
	copy(elements, c.elements)
	return elements, nil
}

// get elements with a specific name. In the event that there are no elements with
// the specified name nil is returned
func (c *Cache[T]) PhotosWithName(ctx context.Context, name string) ([]T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.loadAllUnsafe(ctx); err != nil {
		return nil, err
	}

	if err := c.populateNameMapUnsafe(ctx); err != nil {
		return nil, err
	}

	elementsWithName := c.nameToElements[name]
	elements := make([]T, len(elementsWithName))
	copy(elements, elementsWithName)
	return elements, nil
}

// get the element with the specified ID. In the event that there is no element
// with the specified ID a nil Photo is returned
func (c *Cache[T]) PhotoWithID(ctx context.Context, id types.ID) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.loadAllUnsafe(ctx); err != nil {
		var empty T
		return empty, err
	}

	return c.idToElement[id], nil
}

// Load all elements into the cache. It assumes the mutex guarding the
// cache is already locked.
func (c *Cache[T]) loadAllUnsafe(ctx context.Context) (err error) {
	for page := uint64(0); !c.foundAll; page++ {
		elements, err := c.elementPageFunc(ctx, page)
		if err != nil {
			return err
		}
		if len(elements) == 0 {
			c.foundAll = true
		}
		for _, p := range elements {
			c.addElementUnsafe(p)
		}
	}

	return nil
}

// Add may be called to add a element to the cache. This can be useful when a
// element is created
func (c *Cache[T]) Add(e T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.addElementUnsafe(e)
}

// addElementUnsafe adds a element to the cache. It assumes the mutex guarding the
// cache is already locked.
//
// The nameToPhotos map is not populated as part of this because sometimes
// getting the name of a photo requires a network call (for playlists that were
// not uploaded) In addition as soon as a new photo is added to the cache the
// nameToPhotos map is no longer valid because we may not have a name for that
// photo yet. So we reset the nameToPhotos when adding a new photo to the cache.
func (c *Cache[T]) addElementUnsafe(p T) {

	// If the element is already in the cache just early return
	if _, ok := c.idToElement[p.ID()]; ok {
		return
	}

	c.elements = append(c.elements, p)

	id := p.ID()
	c.idToElement[id] = p

	c.nameToElements = nil
}

func (pc *Cache[T]) populateNameMapUnsafe(ctx context.Context) (err error) {
	if pc.nameToElements != nil {
		return nil
	}

	defer func() {
		if err != nil {
			pc.nameToElements = nil
		}
	}()

	pc.nameToElements = make(map[string][]T)
	for _, p := range pc.elements {
		name, err := p.Name(ctx)
		if err != nil {
			return err
		}
		pc.nameToElements[name] = append(pc.nameToElements[name], p)
	}
	return nil
}

func (c *Cache[T]) Remove(ctx context.Context, e T) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	defer func() {
		if err != nil {
			c.resetUnsafe()
		}
	}()

	// If the element isn't in the cache at all just early return
	cachedPhoto, ok := c.idToElement[e.ID()]
	if !ok {
		return nil
	}

	// Delete element from the pc.elements slice
	id := e.ID()
	for i, possible := range c.elements {
		if id == possible.ID() {
			c.elements[i] = c.elements[len(c.elements)-1]
			c.elements = c.elements[:len(c.elements)-1]
			break
		}
	}

	// Delete the element from the nameToPhotos map / slice
	if c.nameToElements != nil {
		// The element provided to Remove may not be the same element object that we
		// have in memory in the cache. If we have the pc.elementToPhotos then we
		// know that the element object that we have in the cache should know it's
		// name because it had to request it to populate the cache. So lets
		// lookup the element that is in the cache since that should guarantee
		// that we know the name without needing to make a web request to get
		// it.

		name, err := cachedPhoto.Name(ctx)
		if err != nil {
			return err
		}

		s := c.nameToElements[name]
		for i, possible := range s {
			if e.ID() == possible.ID() {
				if len(s) == 1 {
					delete(c.nameToElements, name)
					break
				}
				s[i] = s[len(s)-1]
				s = s[:len(s)-1]
				c.nameToElements[name] = s
				break
			}
		}
	}

	// Delete the photo from the idToPhoto map
	delete(c.idToElement, e.ID())

	return nil
}

// Reset should be called in situations where the cache may no longer be valid
// any more to reset all cache state
func (c *Cache[T]) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetUnsafe()
}

// resetUnsafe does the same as Reset but assumes that the mutex guarding the
// cache is already locked
func (c *Cache[T]) resetUnsafe() {
	c.foundAll = false
	c.elements = nil
	c.nameToElements = nil
	c.idToElement = make(map[types.ID]T)
}
