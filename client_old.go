package nixplay

import (
	"context"
	"io"
)

//xxx I think it would be better to redesign all of this in a more OO way where we have methods on the container / photos

// xxx doc
type ClientOLD interface {
	Containers(ctx context.Context, containerType ContainerType) ([]ContainerOLD, error)

	// Container gets the specified container based on type and name.
	//
	// If the specified container could not be found then ErrContainerNotFound
	// will be returned.
	Container(ctx context.Context, containerType ContainerType, name string) (ContainerOLD, error)

	CreateContainer(ctx context.Context, containerType ContainerType, name string) (ContainerOLD, error)

	DeleteContainer(ctx context.Context, container ContainerOLD) error

	Photos(ctx context.Context, container ContainerOLD) ([]PhotoOLD, error)

	AddPhoto(ctx context.Context, container ContainerOLD, name string, r io.Reader, opts AddPhotoOptions) (PhotoOLD, error)

	//xxx I think I need to add an API to get a photo with a specific name

	DeletePhoto(ctx context.Context, photo PhotoOLD, scope DeleteScope) error
}
