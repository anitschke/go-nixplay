package nixplay

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/anitschke/go-nixplay/auth"
	"github.com/anitschke/go-nixplay/httpx"
)

//xxx there are a few places I use strconv.itoa(int(NUMBER)) for uint64. I
//should switch to using strconv.FormatUint instead



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

func (c *DefaultClient) Containers(ctx context.Context, containerType ContainerType) ([]ContainerOLD, error) {
	switch containerType {
	case AlbumContainerType:
		return c.albums(ctx)
	case PlaylistContainerType:
		return c.playlists(ctx)
	default:
		return nil, ErrInvalidContainerType
	}
}

func (c *DefaultClient) albums(ctx context.Context) ([]ContainerOLD, error) {
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

func (c *DefaultClient) albumsFromURL(ctx context.Context, url string) ([]ContainerOLD, error) {
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

func (c *DefaultClient) playlists(ctx context.Context) ([]ContainerOLD, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.nixplay.com/v3/playlists", bytes.NewReader([]byte{}))
	if err != nil {
		return nil, err
	}

	var playlists playlistsResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &playlists); err != nil {
		return nil, err
	}
	return playlists.ToContainers(), nil

}

func (c *DefaultClient) Container(ctx context.Context, containerType ContainerType, name string) (ContainerOLD, error) {
	//xxx consider adding caching

	containers, err := c.Containers(ctx, containerType)
	if err != nil {
		return ContainerOLD{}, err
	}

	for _, c := range containers {
		if c.Name == name {
			return c, nil
		}
	}

	return ContainerOLD{}, ErrContainerNotFound
}

func (c *DefaultClient) CreateContainer(ctx context.Context, containerType ContainerType, name string) (ContainerOLD, error) {
	switch containerType {
	case AlbumContainerType:
		return c.createAlbum(ctx, name)
	case PlaylistContainerType:
		return c.createPlaylist(ctx, name)
	default:
		return ContainerOLD{}, ErrInvalidContainerType
	}
}

func (c *DefaultClient) createAlbum(ctx context.Context, name string) (ContainerOLD, error) {
	formData := url.Values{
		"name": {name},
	}
	req, err := httpx.NewPostFormRequest(ctx, "https://api.nixplay.com/album/create/json/", formData)
	if err != nil {
		return ContainerOLD{}, err
	}

	var albums albumsResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &albums); err != nil {
		return ContainerOLD{}, err
	}
	if len(albums) != 1 {
		return ContainerOLD{}, errors.New("incorrect number of created containers returned")
	}

	return albums[0].ToContainer(), nil
}

func (c *DefaultClient) createPlaylist(ctx context.Context, name string) (ContainerOLD, error) {

	createRequest := createPlaylistRequest{
		Name: name,
	}
	createBytes, err := json.Marshal(createRequest)
	if err != nil {
		return ContainerOLD{}, err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.nixplay.com/v3/playlists", bytes.NewReader(createBytes))
	if err != nil {
		return ContainerOLD{}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	var createResponse createPlaylistResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &createResponse); err != nil {
		return ContainerOLD{}, err
	}

	// Unfortunately the only data we get back is the playlist ID. So we will
	// just assume that nixplay honored the exact name we asked it to create. I
	// think this should be reasonably safe.
	return ContainerOLD{
		ContainerType: PlaylistContainerType,
		Name:          name,
		ID:            createResponse.PlaylistId,
	}, nil
}

func (c *DefaultClient) DeleteContainer(ctx context.Context, container ContainerOLD) error {
	switch container.ContainerType {
	case AlbumContainerType:
		return c.deleteAlbum(ctx, container)
	case PlaylistContainerType:
		return c.deletePlaylist(ctx, container)
	default:
		return ErrInvalidContainerType
	}
}

func (c *DefaultClient) deleteAlbum(ctx context.Context, container ContainerOLD) error {
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

func (c *DefaultClient) deletePlaylist(ctx context.Context, container ContainerOLD) error {
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

func (c *DefaultClient) Photos(ctx context.Context, container ContainerOLD) ([]PhotoOLD, error) {
	switch container.ContainerType {
	case AlbumContainerType:
		return c.albumPhotos(ctx, container)
	case PlaylistContainerType:
		panic("not implemented") // xxx: Implement
	default:
		return nil, ErrInvalidContainerType
	}
}

func (c *DefaultClient) albumPhotos(ctx context.Context, container ContainerOLD) ([]PhotoOLD, error) {
	var photos []PhotoOLD
	for page := uint64(1); ; page++ {
		photosOnPage, err := c.albumPhotosPage(ctx, container, page)
		if err != nil {
			return nil, err
		}
		if len(photosOnPage) == 0 {
			break
		}
		photos = append(photos, photosOnPage...)
	}
	return photos, nil
}

func (c *DefaultClient) albumPhotosPage(ctx context.Context, container ContainerOLD, page uint64) ([]PhotoOLD, error) {
	limit := 500 //same limit used by nixplay.com when getting photos
	url := fmt.Sprintf("https://api.nixplay.com/album/%d/pictures/json/?page=%d&limit=%d", container.ID, page, limit)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, bytes.NewReader([]byte{}))
	if err != nil {
		return nil, err
	}

	var albumPhotos albumPhotosResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &albumPhotos); err != nil {
		return nil, err
	}

	// xxx make photo an interface and only get size when it is requested
	photos := albumPhotos.ToPhotos()
	for i := range photos {
		photos[i].Size, err = c.getPhotoSize(ctx, photos[i].URL)
		if err != nil {
			return nil, err
		}
	}

	return photos, nil
}

func (c *DefaultClient) getPhotoSize(ctx context.Context, photoURL string) (responseSize uint64, err error) {
	// xxx Getting the size of the photo is a little tricky. Ideally we could
	// use the HEAD method but the way s3 Signature works is it is for a
	// specific method. xxx add rest of details
	//
	// https://stackoverflow.com/a/39663152 curl -v -r 0-0
	//
	// This relies on s3 honoring our request for only a single byte which at
	// the moment it does so I think we can just assume it will continue to do
	// so and not complicate the code more by trying to make it handle future
	// fringe cases where s3 doesn't do what we are expecting.
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to get image size: %w", err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoURL, bytes.NewReader([]byte{})) //xxx consider seeing if I can pass a nil reader in all these spots I am passing empty bytes
	if err != nil {
		return 0, err
	}
	req.Header.Add("Range", "bytes=0-0")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return 0, errors.New(resp.Status)
	}

	// According to the Go doc for Client we must read the body to EOF in order
	// to be able to reuse the TCP connection for subsequent requests. It is
	// only a single byte that we are reading so this is better than not reading
	// and requiring a new request.
	//
	// https://pkg.go.dev/net/http#Client.Do
	//
	//     If the Body is not both read to EOF and closed, the Client's
	//     underlying RoundTripper (typically Transport) may not be able to
	//     re-use a persistent TCP connection to the server for a subsequent
	//     "keep-alive" request.
	bodyByte := make([]byte, 1)
	_, err = resp.Body.Read(bodyByte)
	if err != nil && err != io.EOF {
		return 0, err
	}

	// xxx this header can also get us the content type / mime type. It may be
	// useful to be able to get this in the future?

	contentRange := resp.Header.Get("Content-Range")
	matches := sizeFromContentRangeRegexp.FindStringSubmatch(contentRange)
	if len(matches) != 2 {
		return 0, errors.New("could not parse Content-Range header")
	}
	sizeStr := matches[1]
	return strconv.ParseUint(sizeStr, 10, 64)
}

func (c *DefaultClient) DeletePhoto(ctx context.Context, photo PhotoOLD, scope DeleteScope) error {
	switch photo.parentContainerType {
	case AlbumContainerType:
		if scope == GlobalDeleteScope {
			return errGlobalDeleteScopeNotForAlbums
		}
		return c.deleteAlbumPhoto(ctx, photo)
	case PlaylistContainerType:
		panic("not implemented") // xxx: Implement
	default:
		return ErrInvalidContainerType
	}
}

func (c *DefaultClient) deleteAlbumPhoto(ctx context.Context, photo PhotoOLD) error {
	url := fmt.Sprintf("https://api.nixplay.com/picture/%d/delete/json/", photo.ID)
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
