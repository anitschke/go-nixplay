package nixplay

import (
	"context"
	"errors"
	"io"
)

var (
	ErrContainerNotFound = errors.New("could not find the specified container")
)

type AddPhotoOptions struct {
	// xxx doc optional, if not specified it will be inferred from file
	// extension, if it can not be inferred from extension of file it will throw
	// a documented error.
	//
	// xxx this will use go standard library mime.TypeByExtension, but this is
	// pretty limited in terms of list of extensions supported so we should use
	// mime.AddExtensionType to add in all the image video mime types we can
	// find. OR at least all the ones nixplay supports.
	MIMEType string

	//xxx doc optional, if not specified it will be computed from reader
	FileSize uint64
}

// xxx doc
type Client interface {
	Containers(ctx context.Context, containerType ContainerType) ([]Container, error)

	// Container gets the specified container based on type and name.
	//
	// If the specified container could not be found then ErrContainerNotFound
	// will be returned.
	Container(ctx context.Context, containerType ContainerType, name string) (Container, error)

	CreateContainer(ctx context.Context, containerType ContainerType, name string) (Container, error)

	DeleteContainer(ctx context.Context, container Container) error

	Photos(ctx context.Context, container Container) ([]Photo, error)

	AddPhoto(ctx context.Context, container Container, name string, r io.ReadCloser, opts AddPhotoOptions) (Photo, error)

	DeletePhoto(ctx context.Context, photo Photo) error
}
