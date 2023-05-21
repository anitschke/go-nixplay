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

// xxx move to top
var errGlobalDeleteScopeNotForAlbums = errors.New("global delete scope not currently supported for albums")

// This regexp will parse a content range to give us the full size of the file
// in the range request. It isn't fully compliant with parsing RFC 7233 but the
// other cases for the content range header specified by RFC 7233 don't provide
// the length so I think this is ok for our use case. See
// https://datatracker.ietf.org/doc/html/rfc7233#section-4.2
var sizeFromContentRangeRegexp = regexp.MustCompile(`^bytes \d+-\d+/(\d+)$`)

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
		return Container{}, err
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
	switch container.ContainerType {
	case AlbumContainerType:
		return c.albumPhotos(ctx, container)
	case PlaylistContainerType:
		panic("not implemented") // xxx: Implement
	default:
		return nil, ErrInvalidContainerType
	}
}

func (c *DefaultClient) albumPhotos(ctx context.Context, container Container) ([]Photo, error) {
	var photos []Photo
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

func (c *DefaultClient) albumPhotosPage(ctx context.Context, container Container, page uint64) ([]Photo, error) {
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

func (c *DefaultClient) AddPhoto(ctx context.Context, container Container, name string, r io.Reader, opts AddPhotoOptions) (Photo, error) {
	photoData, r, err := getUploadPhotoData(name, r, opts)
	if err != nil {
		return Photo{}, err
	}

	uploadToken, err := c.getUploadToken(ctx, container)
	if err != nil {
		return Photo{}, err
	}

	uploadNixplayResponse, err := c.uploadNixplay(ctx, container, photoData, uploadToken)
	if err != nil {
		return Photo{}, err
	}

	hasher := md5.New()
	readAndHash := io.TeeReader(r, hasher)

	if err := c.uploadS3(ctx, uploadNixplayResponse, name, readAndHash); err != nil {
		return Photo{}, err
	}

	md5Hash := MD5Hash(hasher.Sum(nil))

	// xxx unfortunately I can't find a way to get the ID of a photo when we do
	// the upload. So we need to resort to asking for all the photos and
	// searching through them until we find the one we uploaded. We might not
	// always need to do this so  I should consider making this part of a
	// different function on the client.
	//
	// I did some experimentation and Nixplay allows two pictures to have the
	// same name but it does NOT allow them to have the same hash. So as we are
	// doing the upload we will hash the file. Then we can get all the photos
	// and detect which is the one we are looking for based on the md5Hash

	//xxx we need to wait for the upload to be done though by looking at
	if len(uploadNixplayResponse.UserUploadIDs) != 1 {
		return Photo{}, errors.New("unable to wait for photo to be uploaded")
	}
	monitorId := uploadNixplayResponse.UserUploadIDs[0]
	if err := c.monitorUpload(ctx, monitorId); err != nil {
		return Photo{}, err
	}

	photos, err := c.Photos(ctx, container)
	if err != nil {
		return Photo{}, err
	}
	for _, p := range photos {
		if p.MD5Hash == md5Hash {
			return p, nil
		}
	}
	return Photo{}, errors.New("unable to find photo after upload")
}

type uploadPhotoData struct {
	AddPhotoOptions
	Name string
}

func getUploadPhotoData(name string, r io.Reader, opts AddPhotoOptions) (uploadPhotoData, io.Reader, error) {
	data := uploadPhotoData{
		AddPhotoOptions: opts,
		Name:            name,
	}

	if data.MIMEType == "" {
		ext := filepath.Ext(name)
		if ext == "" {
			return uploadPhotoData{}, nil, fmt.Errorf("could not determine file extension for file %q", name)
		}
		data.MIMEType = mime.TypeByExtension(ext)
		if data.MIMEType == "" {
			return uploadPhotoData{}, nil, fmt.Errorf("could not determine mime type for file %q", name)
		}
	}

	// If we don't know the file size we will first try to use seeker APIs to
	// get the size since that is most efficient. If that doesn't work we will
	// resort to reading into a buffer which requires us to buffer the entire
	// file into memory, not ideal.
	if data.FileSize == 0 {
		if s, ok := r.(io.Seeker); ok {
			size, err := s.Seek(0, io.SeekEnd)
			if err != nil {
				return uploadPhotoData{}, nil, err
			}
			// seek back to the start of file so that it can be read again properly
			if _, err := s.Seek(0, io.SeekStart); err != nil {
				return uploadPhotoData{}, nil, err
			}
			data.FileSize = uint64(size)
		} else {
			buf := new(bytes.Buffer)
			size, err := buf.ReadFrom(r)
			if err != nil {
				return uploadPhotoData{}, nil, err
			}
			data.FileSize = uint64(size)
			r = buf
		}
	}

	return data, r, nil
}

func uploadTokenForm(container Container) (url.Values, error) {
	switch container.ContainerType {
	case AlbumContainerType:
		return url.Values{
			"albumId": {strconv.Itoa(int(container.ID))},
			"total":   {"1"},
		}, nil
	case PlaylistContainerType:
		return url.Values{
			"playlistId": {strconv.Itoa(int(container.ID))},
			"total":      {"1"},
		}, nil
	default:
		return nil, ErrInvalidContainerType
	}
}

func (c *DefaultClient) getUploadToken(ctx context.Context, container Container) (returnedToken string, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error getting upload token: %w", err)
		}
	}()

	form, err := uploadTokenForm(container)
	if err != nil {
		return "", err
	}

	req, err := httpx.NewPostFormRequest(ctx, "https://api.nixplay.com/v3/upload/receivers/", form)
	if err != nil {
		return "", err
	}

	var response uploadTokenResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &response); err != nil {
		return "", err
	}

	return response.Token, nil
}

func uploadNixplayForm(container Container, photo uploadPhotoData, token string) (url.Values, error) {
	form := url.Values{
		"uploadToken": {token},
		"fileName":    {photo.Name},
		"fileType":    {photo.MIMEType},
		"fileSize":    {strconv.Itoa(int(photo.FileSize))},
	}

	switch container.ContainerType {
	case AlbumContainerType:
		form.Add("albumId", strconv.Itoa(int(container.ID)))
	case PlaylistContainerType:
		form.Add("playlistId", strconv.Itoa(int(container.ID)))
	default:
		return nil, ErrInvalidContainerType
	}

	return form, nil
}

func (c *DefaultClient) uploadNixplay(ctx context.Context, container Container, photo uploadPhotoData, token string) (returnedResponse uploadNixplayResponse, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error uploading to nixplay: %w", err)
		}
	}()

	form, err := uploadNixplayForm(container, photo, token)
	if err != nil {
		return uploadNixplayResponse{}, err
	}

	req, err := httpx.NewPostFormRequest(ctx, "https://api.nixplay.com/v3/photo/upload/", form)
	if err != nil {
		return uploadNixplayResponse{}, err
	}

	var response uploadNixplayResponseContainer
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &response); err != nil {
		return uploadNixplayResponse{}, err
	}

	return response.Data, nil
}

func (c *DefaultClient) uploadS3(ctx context.Context, u uploadNixplayResponse, filename string, r io.Reader) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("error uploading to s3 bucket: %w", err)
		}
	}()

	reqBody := &bytes.Buffer{}
	writer := multipart.NewWriter(reqBody)

	formVals := map[string]string{
		"key":                        u.Key,
		"acl":                        u.ACL,
		"content-type":               u.FileType,
		"x-amz-meta-batch-upload-id": u.BatchUploadID,
		"success_action_status":      "201",
		"AWSAccessKeyId":             u.AWSAccessKeyID,
		"Policy":                     u.Policy,
		"Signature":                  u.Signature,
	}
	for k, v := range formVals {
		w, err := writer.CreateFormField(k)
		if err != nil {
			return err
		}
		io.WriteString(w, v)
	}

	w, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, r)
	if err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.S3UploadURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("content-type", fmt.Sprintf("multipart/form-data; boundary=%s", writer.Boundary()))
	req.Header.Set("origin", "https://app.nixplay.com")
	req.Header.Set("referer", "https://app.nixplay.com")
	resp, err := http.DefaultClient.Do(req) // xxx don't use the deafult client, use the not-authourized one we were provided
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return fmt.Errorf("error uploading: %s", resp.Status)
	}
	return nil
}

func (c *DefaultClient) monitorUpload(ctx context.Context, monitorID string) (err error) {
	defer func() { //xxx do this sort of thing in more places
		if err != nil {
			err = fmt.Errorf("error monitoring upload: %w", err)
		}
	}()

	url := fmt.Sprintf("https://upload-monitor.nixplay.com/status?id=%s", monitorID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}
	resp, err := c.authClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(resp.Status)
	}
	return nil
}

func (c *DefaultClient) DeletePhoto(ctx context.Context, photo Photo, scope DeleteScope) error {
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

func (c *DefaultClient) deleteAlbumPhoto(ctx context.Context, photo Photo) error {
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
