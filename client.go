package nixplay

import (
	"context"
	"errors"
	"io"

	"github.com/anitschke/go-nixplay/types"
)

var (
	ErrContainerNotFound = errors.New("could not find the specified container")
)

type AddPhotoOptions struct {
	// xxx doc optional, if not specified it will be inferred from file
	// extension, if it can not be inferred from extension of file it will throw
	// a documented error.
	//
	// xxx doc this will use go standard library mime.TypeByExtension, but this is
	// pretty limited in terms of list of extensions supported so we should use
	// mime.AddExtensionType to add in all the image video mime types we can
	// find. OR at least all the ones nixplay supports.
	MIMEType string

	//xxx doc optional, if not specified it will be computed from reader
	FileSize int64
}

// xxx doc Add comment in limitations section of doc that it is possible to remove a
// photo from an playlist without removing it from the album it lives in but for
// the purpose of keeping things simple right now we don't support that. We just
// always delete the photo from everywhere.
//
// xxx doc nixplay has a few different flavors of delete. For albums it looks
// like you can only delete. but for playlists it looks like you can choose to
// totally delete the photo, or remove it from the playlist but keep it around
// in the album it belongs in.
//
// I did some playing around and there is also some weird and buggy feeling
// behavior. If you choose the "permanently  delete" option in playlist it will
// remove ALL instances of that photo if it exists in multiple albums and not
// just from the one album it was added from. This happens even if you manually
// upload the photo multiple times to different albums instead of using
// Nixplay's copy to album option. This is in contrast to deleting a photo from
// a album where the only option is to remove it from that one album.
//
// The sort of exception to this is that photos are owned by a album and
// playlists are only associated to a photo, so if you delete a photo from an
// album then it will also be removed from any playlists it was a part of.
//
// Given all of this I think the easiest thing to do is to use a flavor of
// delete where we only remove the photo from the container you got it from
// instead of doing a more global delete of it. This should give relatively
// consistent behavior regardless of what sort of container it is coming from.
//
// The downside of the above easiest option is that it means that if I setup
// rclone to just sync a playlist, then when a photo is deleted from the
// playlist it will essentially "leak" the photo in the downloads folder and
// that could bloat memory usage to the point where I might start running out of
// storage space if stuff changes often. I think the answer to this is have a
// "DeleteScope" option that says at what scope the file will be deleted, either
// global or local to playlist. Then setup rsync where there is an option that
// lets you pick how delete of photos in a playlist will be handled.
//
// All this means that at the moment we will only support GlobalDeleteScope for
// playlists and not albums. If we really wanted we could support
// GlobalDeleteScope by getting a list of all the photos, comparing the md5 hash
// and deleting any that have the same hash. But this would be expensive... so
// lets just error out for now if someone tries to use global for deleting a
// photo from an album.

//xxx no guarantee that IDs will remain the same for same objects in the future

// xxx doc
type Client interface {
	Containers(ctx context.Context, containerType types.ContainerType) ([]Container, error)

	// Container gets the specified container based on type and name.
	//
	// If the specified container could not be found then ErrContainerNotFound
	// will be returned.
	//
	// xxx I don't need ErrContainerNotFound anymore since I can now just return a nil container
	Container(ctx context.Context, containerType types.ContainerType, name string) (Container, error)

	CreateContainer(ctx context.Context, containerType types.ContainerType, name string) (Container, error)
}

// xxx doc
type Container interface {
	ID() types.ID
	ContainerType() types.ContainerType

	Name(ctx context.Context) (string, error)
	PhotoCount(ctx context.Context) (int64, error)
	Photos(ctx context.Context) ([]Photo, error)
	PhotosWithName(ctx context.Context, name string) ([]Photo, error)
	PhotoWithID(ctx context.Context, id types.ID) (Photo, error) //xxx doc if no photo found then return nil

	Delete(ctx context.Context) error
	AddPhoto(ctx context.Context, name string, r io.Reader, opts AddPhotoOptions) (Photo, error)

	// xxx doc May be called to reset the Containers internal cache of photos
	ResetCache()
}

// xxx doc
type Photo interface {
	ID() types.ID

	Name(ctx context.Context) (string, error)
	Size(ctx context.Context) (int64, error)
	MD5Hash(ctx context.Context) (types.MD5Hash, error)
	URL(ctx context.Context) (string, error)

	Open(ctx context.Context) (io.ReadCloser, error)
	Delete(ctx context.Context) error
}
