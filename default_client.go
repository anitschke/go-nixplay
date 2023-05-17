package nixplay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

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
		return c.playlists(ctx)
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

func (c *DefaultClient) playlists(ctx context.Context) ([]Container, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.nixplay.com/v3/playlists", bytes.NewReader([]byte{}))
	if err != nil {
		return nil, err
	}

	var playlists playlistResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &playlists); err != nil {
		return nil, err
	}
	return playlists.ToContainers(), nil

}

func (c *DefaultClient) Container(ctx context.Context, containerType ContainerType, name string) (Container, error) {
	//xxx consider adding caching

	containers, err := c.Containers(ctx, containerType)
	if err != nil {
		return Container{}, err
	}

	for _, c := range containers {
		if c.Name == name {
			return c, nil
		}
	}

	return Container{}, ErrContainerNotFound
}

func (c *DefaultClient) CreateContainer(ctx context.Context, containerType ContainerType, name string) (Container, error) {
	switch containerType {
	case AlbumContainerType:
		return c.createAlbum(ctx, name)
	case PlaylistContainerType:
		return c.createPlaylist(ctx, name)
	default:
		return Container{}, ErrInvalidContainerType
	}
}

func (c *DefaultClient) createAlbum(ctx context.Context, name string) (Container, error) {
	formData := url.Values{
		"name": {name},
	}
	req, err := httpx.NewPostFormRequest(ctx, "https://api.nixplay.com/album/create/json/", formData)
	if err != nil {
		return Container{}, nil
	}

	var albums albumsResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &albums); err != nil {
		return Container{}, err
	}
	if len(albums) != 1 {
		return Container{}, errors.New("incorrect number of created containers returned")
	}

	return albums[0].ToContainer(), nil
}

func (c *DefaultClient) createPlaylist(ctx context.Context, name string) (Container, error) {

	createRequest := createPlaylistRequest{
		Name: name,
	}
	createBytes, err := json.Marshal(createRequest)
	if err != nil {
		return Container{}, err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.nixplay.com/v3/playlists", bytes.NewReader(createBytes))
	if err != nil {
		return Container{}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	var createResponse createPlaylistResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &createResponse); err != nil {
		return Container{}, err
	}

	// Unfortunately the only data we get back is the playlist ID. So we will
	// just assume that nixplay honored the exact name we asked it to create. I
	// think this should be reasonably safe.
	return Container{
		ContainerType: PlaylistContainerType,
		Name:          name,
		ID:            createResponse.PlaylistId,
	}, nil
}

func (c *DefaultClient) DeleteContainer(ctx context.Context, container Container) error {
	switch container.ContainerType {
	case AlbumContainerType:
		return c.deleteAlbum(ctx, container)
	case PlaylistContainerType:
		return c.deletePlaylist(ctx, container)
	default:
		return ErrInvalidContainerType
	}
}

func (c *DefaultClient) deleteAlbum(ctx context.Context, container Container) error {
	url := fmt.Sprintf("https://api.nixplay.com/album/%d/delete/json/", container.ID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}
	resp, err := c.authClient.Do(req)
	if err != nil {
		return err
	}
	//xxx check response code
	resp.Body.Close()
	return nil
}

func (c *DefaultClient) deletePlaylist(ctx context.Context, container Container) error {
	url := fmt.Sprintf("https://api.nixplay.com/v3/playlists/%d", container.ID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}
	resp, err := c.authClient.Do(req)
	if err != nil {
		return err
	}
	//xxx check response code
	resp.Body.Close()
	return nil
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
