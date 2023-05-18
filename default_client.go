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
	"strconv"

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

	var photos albumPhotosResponse
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &photos); err != nil {
		return nil, err
	}
	return photos.ToPhotos(), nil
}

func (c *DefaultClient) AddPhoto(ctx context.Context, container Container, name string, r io.ReadCloser, opts AddPhotoOptions) (Photo, error) {
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
	photos, err := c.Photos(ctx, container)
	if err != nil {
		return Photo{}, nil
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

func getUploadPhotoData(name string, r io.ReadCloser, opts AddPhotoOptions) (uploadPhotoData, io.ReadCloser, error) {
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
			// seek back to the start of file so that it can be served properly
			if _, err := s.Seek(0, io.SeekStart); err != nil {
				return uploadPhotoData{}, nil, err
			}
			data.FileSize = uint64(size)
		} else {
			// xxx what if the expected behavior for read closers. Is it expected
			// that we will always close them even if we error out? If that is
			// the case should this defer happen right at the outer most call to
			// add the photo?
			//
			// I think it is an anti-pattern to pass in a ReadCloser like this.
			// It makes it confusing what behavior will be in cases like this
			// where we are erroring out. It would be better to just accept a
			// Reader. Then the user can just follow the normal pattern and do a
			// "defer r.Close()" in their own code right before they pass the
			// reader into the AddPhotoAPI.
			defer r.Close()
			buf := new(bytes.Buffer)
			size, err := buf.ReadFrom(r)
			if err != nil {
				return uploadPhotoData{}, nil, err
			}
			data.FileSize = uint64(size)
			r = io.NopCloser(buf)
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

func (c *DefaultClient) getUploadToken(ctx context.Context, container Container) (string, error) {
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

func (c *DefaultClient) uploadNixplay(ctx context.Context, container Container, photo uploadPhotoData, token string) (uploadNixplayResponse, error) {
	form, err := uploadNixplayForm(container, photo, token)
	if err != nil {
		return uploadNixplayResponse{}, err
	}

	req, err := httpx.NewPostFormRequest(ctx, "https://api.nixplay.com/v3/upload/receivers/", form)
	if err != nil {
		return uploadNixplayResponse{}, err
	}

	var response uploadNixplayResponseContainer
	if err := httpx.DoUnmarshalJSONResponse(c.authClient, req, &response); err != nil {
		return uploadNixplayResponse{}, err
	}

	return response.Data, nil
}

func (c *DefaultClient) uploadS3(ctx context.Context, u uploadNixplayResponse, filename string, r io.Reader) error {

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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return fmt.Errorf("error uploading: %s", resp.Status)
	}
	return nil
}

func (c *DefaultClient) DeletePhoto(ctx context.Context, photo Photo) error {
	// xxx add "DeleteScope" nixplay has a few different flavors of delete. For
	// albums it looks like you can only delete. but for playlists it looks like
	// you can choose to totally delete the photo, or remove it from the
	// playlist but keep it around in the album it belongs in.
	//
	// I did some playing around and there is also some weird and buggy
	// behavior. If you choose the "permanently  delete" option in playlist it
	// will remove ALL instances of that photo if it exists in multiple albums
	// and not just from the one album it was added from. This happens even if
	// you manually upload the photo multiple times to different albums instead
	// of using Nixplay's copy to album option. This is in contrast to deleting
	// a photo from a playlist where the only option is to remove it from that
	// one album.
	//
	// The sort of exception to this is that photos are owned by a album and
	// playlists are only associated to a photo, so if you delete a photo from
	// an album then it will also be removed from any playlists it was a part
	// of.
	//
	// Given all of this I think the easiest thing to do is to use a flavor of
	// delete where we only remove the photo from the container you got it from
	// instead of doing a more global delete of it. This should give relatively
	// consistent behavior regardless of what sort of container it is coming
	// from.
	//
	// The downside of the above easiest option is that it means that if I setup
	// rclone to just sync a playlist, then when a photo is deleted from the
	// playlist it will essentially "leak" the photo in the downloads folder and
	// that could bloat memory usage to the point where I might start running
	// out of storage space if stuff changes often. I think the answer to this
	// is have a "DeleteScope" option that says at what scope the file will be
	// deleted, either global or local to playlist. Then setup rsync where there
	// is an option that lets you pick how delete of photos in a playlist will
	// be handled.

	// xxx doc in Nixplay playlists are allowed to contain multiple copies of
	// the same photo, either from the same album, or from different albums.
	// When a photo is deleted it will remove all copies of that photo. This
	// starts getting tricky to support. I think I should just document
	// somewhere that this tool won't support multiple copies of the same photo
	// in an playlist and if you try to do this you may get buggy results.

	panic("not implemented") // xxx: Implement
}
