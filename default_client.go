package nixplay

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/anitschke/go-nixplay/auth"
	"github.com/anitschke/go-nixplay/httpx"
)

// xxx doc
type DefaultClientOptions struct {
	// xxx doc optional
	HTTPClient httpx.Client
}

type DefaultClient struct {
	client     httpx.Client
	authClient httpx.Client
}

var _ = (Client)((*DefaultClient)(nil))

func NewDefaultClient(ctx context.Context, a auth.Authorization, opts DefaultClientOptions) (*DefaultClient, error) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{}
	}

	authClient, err := auth.NewAuthorizedClient(ctx, opts.HTTPClient, a)
	if err != nil {
		return nil, fmt.Errorf("authorization failed: %w", err)
	}

	return &DefaultClient{
		client:     opts.HTTPClient,
		authClient: authClient,
	}, nil
}

func (c *DefaultClient) Containers(ctx context.Context, containerType ContainerType) ([]Container, error) {
	switch containerType {
	case AlbumContainerType:
		return c.albums(ctx)
	case PlaylistContainerType:
		panic("not implemented") // xxx: Implement
	default:
		return nil, ErrInvalidContainerType
	}
}

func (c *DefaultClient) albums(ctx context.Context) ([]Container, error) {
	webAlbums, err := c.albumsFromURL(ctx, "https://api.nixplay.com/v2/albums/web/json/")
	if err != nil {
		return nil, err
	}
	emailAlbums, err := c.albumsFromURL(ctx, "https://api.nixplay.com/v2/albums/email/json/")
	if err != nil {
		return nil, err
	}
	return append(webAlbums, emailAlbums...), nil
}

func (c *DefaultClient) albumsFromURL(ctx context.Context, url string) ([]Container, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, bytes.NewReader([]byte{}))
	if err != nil {
		return nil, err
	}

	var albums albumsResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &albums); err != nil {
		return nil, err
	}
	return albums.ToContainers(), nil
}

// Container gets the specified container based on type and name.
//
// If the specified container could not be found then ErrContainerNotFound
// will be returned.
func (c *DefaultClient) Container(ctx context.Context, containerType ContainerType, name string) (Container, error) {
	panic("not implemented") // xxx: Implement
}

func (c *DefaultClient) CreateContainer(ctx context.Context, containerType ContainerType, name string) (Container, error) {
	panic("not implemented") // xxx: Implement
}

func (c *DefaultClient) DeleteContainer(ctx context.Context, container Container) error {
	panic("not implemented") // xxx: Implement
}

func (c *DefaultClient) Photos(ctx context.Context, container Container) ([]Photo, error) {
	panic("not implemented") // xxx: Implement
}

func (c *DefaultClient) AddPhoto(ctx context.Context, container Container, name string, r io.ReadCloser, opts AddPhotoOptions) (Photo, error) {
	panic("not implemented") // xxx: Implement
}

func (c *DefaultClient) DeletePhoto(ctx context.Context, photo Photo) error {
	panic("not implemented") // xxx: Implement
}
