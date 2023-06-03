package nixplay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/anitschke/go-nixplay/httpx"
	"github.com/anitschke/go-nixplay/internal/auth"
	"github.com/anitschke/go-nixplay/types"
)

//xxx move all the extra crap in the nixplay package into an internal package, things are starting to get messy

// xxx doc
type DefaultClientOptions struct {
	// xxx doc optional
	HTTPClient httpx.Client
}

type DefaultClient struct {
	client httpx.Client
}

var _ = (Client)((*DefaultClient)(nil))

func NewDefaultClient(ctx context.Context, a auth.Authorization, opts DefaultClientOptions) (*DefaultClient, error) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{}
	}

	client, err := auth.NewAuthorizedClient(ctx, opts.HTTPClient, a)
	if err != nil {
		return nil, fmt.Errorf("authorization failed: %w", err)
	}

	return &DefaultClient{
		client: client,
	}, nil
}

func (c *DefaultClient) Containers(ctx context.Context, containerType types.ContainerType) ([]Container, error) {
	switch containerType {
	case types.AlbumContainerType:
		return c.albums(ctx)
	case types.PlaylistContainerType:
		return c.playlists(ctx)
	default:
		return nil, types.ErrInvalidContainerType
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
	if err := httpx.DoUnmarshalJSONResponse(c.client, req, &albums); err != nil {
		return nil, err
	}
	return albums.ToContainers(c.client), nil
}

func (c *DefaultClient) playlists(ctx context.Context) ([]Container, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.nixplay.com/v3/playlists", bytes.NewReader([]byte{}))
	if err != nil {
		return nil, err
	}

	var playlists playlistsResponse
	if err := httpx.DoUnmarshalJSONResponse(c.client, req, &playlists); err != nil {
		return nil, err
	}
	return playlists.ToContainers(c.client), nil

}

func (c *DefaultClient) Container(ctx context.Context, containerType types.ContainerType, name string) (Container, error) {
	//xxx consider adding caching

	containers, err := c.Containers(ctx, containerType)
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		if c.Name() == name {
			return c, nil
		}
	}

	return nil, ErrContainerNotFound
}

func (c *DefaultClient) CreateContainer(ctx context.Context, containerType types.ContainerType, name string) (Container, error) {
	switch containerType {
	case types.AlbumContainerType:
		return c.createAlbum(ctx, name)
	case types.PlaylistContainerType:
		return c.createPlaylist(ctx, name)
	default:
		return nil, types.ErrInvalidContainerType
	}
}

func (c *DefaultClient) createAlbum(ctx context.Context, name string) (Container, error) {
	formData := url.Values{
		"name": {name},
	}
	req, err := httpx.NewPostFormRequest(ctx, "https://api.nixplay.com/album/create/json/", formData)
	if err != nil {
		return nil, err
	}

	var albums albumsResponse
	if err := httpx.DoUnmarshalJSONResponse(c.client, req, &albums); err != nil {
		return nil, err
	}
	if len(albums) != 1 {
		return nil, errors.New("incorrect number of created containers returned")
	}

	return albums[0].ToContainer(c.client), nil
}

func (c *DefaultClient) createPlaylist(ctx context.Context, name string) (Container, error) {

	createRequest := createPlaylistRequest{
		Name: name,
	}
	createBytes, err := json.Marshal(createRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.nixplay.com/v3/playlists", bytes.NewReader(createBytes))
	if err != nil {
		return nil, nil
	}
	req.Header.Set("Content-Type", "application/json")

	var createResponse createPlaylistResponse
	if err := httpx.DoUnmarshalJSONResponse(c.client, req, &createResponse); err != nil {
		return nil, err
	}

	// Unfortunately the only data we get back is the playlist ID. So we will
	// just assume that nixplay honored the exact name we asked it to create. I
	// think this should be reasonably safe.
	nPhotos := int64(0)
	return newPlaylist(c.client, name, createResponse.PlaylistId, nPhotos), nil
}
