package nixplay

import (
	"context"
	"io"

	_ "github.com/anitschke/go-nixplay/internal/mime"
	"github.com/anitschke/go-nixplay/types"
)

// AddPhotoOptions are optional arguments may be specified when adding photos to
// Nixplay.
type AddPhotoOptions struct {
	// MIMEType of the photo to be uploaded.
	//
	// Specifying the MIME Type is optional. However Nixplay does require that
	// the MIME Type is provided, so if a MIME Type is not specified then one
	// will be inferred from the file extension.
	//
	// According to Nixplay documentation  JPEG, PNG, TIFF, HEIC, MP4 are all
	// supported see the following for more details:
	// https://web.archive.org/web/20230328184513/https://support.nixplay.com/hc/en-us/articles/900002393886-What-photo-and-video-formats-does-Nixplay-support-
	//
	// If you try to upload an unsupported file type you will get a 400 Bad
	// Request error from the server.
	MIMEType string

	// FileSize in bytes of the photo to be uploaded to Nixplay.
	//
	// Specifying the MIME Type is optional. However Nixplay does require that
	// the file size is provided, so if the the size is not specified then it
	// will be computed based on the io.Reader provided. An attempt will be made
	// to efficiently compute the size without buffering the entire photo into
	// memory however in some cases it may be necessary to buffer the full photo
	// into memory.
	FileSize int64
}

// Client is the interface that is essentially the entrypoint into communicating
// with Nixplay. It provides the ability to query containers (albums or
// playlists) or create new containers.
type Client interface {

	// Containers gets all containers of the specified ContainerType
	Containers(ctx context.Context, containerType types.ContainerType) ([]Container, error)

	// ContainersWithName gets a containers based on type and name.
	//
	// If no containers with the specified name could be found then an empty
	// slice of containers will be returned.
	ContainersWithName(ctx context.Context, containerType types.ContainerType, name string) ([]Container, error)

	// ContainerWithName gets the container based on type and unique name as
	// returned by Container.NameUnique.
	//
	// If no container with the specified unique name could be found then a nil
	// Container will be returned.
	ContainerWithUniqueName(ctx context.Context, containerType types.ContainerType, name string) (Container, error)

	// CreateContainer creates a container of the specified type and name.
	CreateContainer(ctx context.Context, containerType types.ContainerType, name string) (Container, error)

	// Reset cache resets the internal cache of containers
	//
	// For more details see https://github.com/anitschke/go-nixplay/#caching
	ResetCache()
}

// Container is the interface for an object that contains photos, either an
// album or playlist.
type Container interface {
	// ID is a unique identifier for the container. This identifier is
	// guaranteed to stable across go-nixplay sessions although the identifier
	// for a given container may change with upgrades to go-nixplay. Note that
	// this identifier may be different than the internal identifier used by
	// Nixplay to identifier an album or playlist.
	ID() types.ID

	ContainerType() types.ContainerType

	Name(ctx context.Context) (string, error)
	// NameUnique returns a name that has an additional unique ID appended to
	// the end of the name if there are containers of the same type. If there
	// are no containers with the same name and of the same type then NameUnique
	// returns the same thing as Name.
	NameUnique(ctx context.Context) (string, error)

	// PhotoCount gets the number of photos within the container.
	//
	// Note that this API is often times more efficient than len(c.Photos)
	PhotoCount(ctx context.Context) (int64, error)

	// Photos gets all photos in the container
	Photos(ctx context.Context) ([]Photo, error)

	// PhotosWithName gets all photos in the container with the specified name.
	PhotosWithName(ctx context.Context, name string) ([]Photo, error)

	// PhotoWithUniqueName gets the photo in the container with the unique name
	// as returned by Photo.NameUnique
	PhotoWithUniqueName(ctx context.Context, name string) (Photo, error)

	// PhotoWithID gets the photo in the container with the specified ID.
	//
	// If no photo with the specified ID can be found in the container nil is
	// returned.
	PhotoWithID(ctx context.Context, id types.ID) (Photo, error)

	// Delete deletes the container.
	//
	// See
	// https://github.com/anitschke/go-nixplay/#photo-additiondelete-is-not-atomic
	// for further discussion of delete behavior.
	Delete(ctx context.Context) error
	AddPhoto(ctx context.Context, name string, r io.Reader, opts AddPhotoOptions) (Photo, error)

	// Reset cache resets the internal cache of photos
	//
	// For more details see https://github.com/anitschke/go-nixplay/#caching
	ResetCache()
}

// Photo is an interface for an object that represents a photo. Even though a
// photo may exist in one album and multiple playlists the Photo object
// represents a photo within the specific Container object that it was obtained
// from.
type Photo interface {
	// ID is a unique identifier for the photo. This identifier is guaranteed to
	// stable across go-nixplay sessions although the identifier for a given
	// container may change with upgrades to go-nixplay. Note that this
	// identifier may be different than the internal identifier used by Nixplay
	// to identifier a photo.
	//
	// Note copies of the same photo in different containers will have a unique
	// identifier.
	//
	// Note that duplicate copies of the same photo within the same playlist
	// will have the same identifier. See further discussion of this issue in
	// https://github.com/anitschke/go-nixplay/#multiple-copies-of-photos-in-playlist
	ID() types.ID

	Name(ctx context.Context) (string, error)

	// NameUnique returns a name that has an additional unique ID appended to
	// the end of the name if there are photos with the same name in the
	// container that this photo resides in. If there are no photos with the
	// same name in the container then NameUnique returns the same thing as
	// Name.
	NameUnique(ctx context.Context) (string, error)

	Size(ctx context.Context) (int64, error)
	MD5Hash(ctx context.Context) (types.MD5Hash, error)

	// URL returns the URL for the original photo that was uploaded to Nixplay.
	URL(ctx context.Context) (string, error)

	// Open opens the photo for reading the contents of the photo.
	Open(ctx context.Context) (io.ReadCloser, error)

	// Delete deletes the photo from the parent container that this photo object
	// was obtained from.
	//
	// See
	// https://github.com/anitschke/go-nixplay/#photo-additiondelete-is-not-atomic
	// for further discussion of delete behavior.
	Delete(ctx context.Context) error
}
