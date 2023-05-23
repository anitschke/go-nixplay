package nixplay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/anitschke/go-nixplay/httpx"
)

var errGlobalDeleteScopeNotForAlbums = errors.New("global delete scope not currently supported for albums")

// This regexp will parse a content range to give us the full size of the file
// in the range request. It isn't fully compliant with parsing RFC 7233 but the
// other cases for the content range header specified by RFC 7233 don't provide
// the length so I think this is ok for our use case. See
// https://datatracker.ietf.org/doc/html/rfc7233#section-4.2
var sizeFromContentRangeRegexp = regexp.MustCompile(`^bytes \d+-\d+/(\d+)$`)

type albumPhoto struct {
	name    string
	id      ID
	md5Hash MD5Hash

	album      Container
	authClient httpx.Client
	client     httpx.Client

	nixplayID uint64
	size      int64
	url       string
}

// xxx doc nixplayID should be zero if not known
// xxx doc size should be -1 if not known
func newAlbumPhoto(album Container, authClient httpx.Client, client httpx.Client, name string, md5Hash MD5Hash, nixplayID uint64, size int64, url string) *albumPhoto {

	// Unfortunately when we upload a photo there isn't any way to get the ID of
	// the photo without getting ALL the photos in that album and searching
	// through them all. But ideally we want some sort of identifier for an
	// image that is stable without having to do this. Nixplay allows duplicate
	// named pictures int he same album but does not allow two copies of the
	// same picture. So what we will do is compute the ID to be the hash album
	// ID + MD5Hash of the image. This lets us have a stable ID for the image
	// even if we don't know what Nixplay's internal identifier is.

	albumID := album.ID()
	hasher := sha256.New()
	hasher.Write(albumID[:]) // shouldn't ever error so we don't need to check for one
	hasher.Write(md5Hash[:])
	id := ID(hasher.Sum([]byte{}))

	return &albumPhoto{
		name:    name,
		id:      id,
		md5Hash: md5Hash,

		album:      album,
		authClient: authClient,
		client:     client,

		nixplayID: nixplayID,
		size:      size,
		url:       url,
	}
}

var _ = (Photo)((*albumPhoto)(nil))

func (a *albumPhoto) Name() string {
	return a.name
}

func (a *albumPhoto) ID() ID {
	return a.id
}

func (a *albumPhoto) Size(ctx context.Context) (int64, error) {
	if a.size == -1 {
		err := a.populatePhotoDataFromHead(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get image size: %w", err)
		}
	}
	if a.size == -1 {
		return 0, errors.New("unable to determine photo size")
	}

	return a.size, nil
}

func (a *albumPhoto) MD5Hash(ctx context.Context) (MD5Hash, error) {
	return a.md5Hash, nil
}

func (a *albumPhoto) URL(ctx context.Context) (string, error) {
	if a.url == "" {
		if err := a.populatePhotoDataFromListSearch(ctx); err != nil {
			return "", fmt.Errorf("failed to get image url: %w", err)
		}
	}
	if a.url == "" {
		return "", errors.New("unable to determine photo URL")
	}
	return a.url, nil
}

func (a *albumPhoto) Open(ctx context.Context) (retReadCloser io.ReadCloser, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to open photo: %w", err)
		}
	}()

	photoURL, err := a.URL(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoURL, bytes.NewReader([]byte{})) //xxx consider seeing if I can pass a nil reader in all these spots I am passing empty bytes
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		_, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(resp.Status)
	}

	if a.size == -1 {
		sizeStr := resp.Header.Get("Content-Length")
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return nil, err
		}
		a.size = size
	}

	return resp.Body, nil
}

func (a *albumPhoto) Delete(ctx context.Context, scope DeleteScope) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to delete photo: %w", err)
		}
	}()

	if scope == GlobalDeleteScope {
		return errGlobalDeleteScopeNotForAlbums
	}

	nixplayID, err := a.getNixplayID(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.nixplay.com/picture/%d/delete/json/", nixplayID)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}
	resp, err := a.authClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read body so we can reuse http transport later
	if _, err := io.ReadAll(resp.Body); err != nil {
		return err
	}

	if err := httpx.StatusError(resp); err != nil {
		return err
	}

	return nil
}

func (a *albumPhoto) getNixplayID(ctx context.Context) (uint64, error) {
	if a.nixplayID == 0 {
		if err := a.populatePhotoDataFromListSearch(ctx); err != nil {
			return 0, fmt.Errorf("failed to get internal Nixplay ID: %w", err)
		}
	}
	if a.nixplayID == 0 {
		return 0, errors.New("unable to determine internal Nixplay ID")
	}
	return a.nixplayID, nil
}

func (a *albumPhoto) populatePhotoDataFromListSearch(ctx context.Context) error {
	// Unfortunately when we add a new photo there doesn't seem to be any API to
	// get nixplay's ID or URL for the photo. So what we need to do is query the
	// parent album for all it's photos and then search for this photo by
	// looking for one that has the same md5hash since we can compute and store
	// that when we are doing the upload. (we can't match by name because
	// nixplay allows multiple files with the same name.)

	// xxx Photos needs to do pagination, there is no need to go through all
	// the pages if we are only looking for a photo in the first page. I should
	// add a Walk API to container that lets you quit searching after you have
	// found what you are looking for.
	photos, err := a.album.Photos(ctx)
	if err != nil {
		return err
	}
	for _, p := range photos {
		ap, ok := p.(*albumPhoto)
		if !ok {
			return errors.New("invalid photo when attempting to populate from list")
		}
		if ap.md5Hash == a.md5Hash {
			if ap.nixplayID == 0 || ap.url == "" {
				return errors.New("incomplete photo data in list")
			}
			a.nixplayID = ap.nixplayID
			a.url = ap.url
			return nil
		}
	}
	return errors.New("unable to find photo to get required data")
}

func (a *albumPhoto) populatePhotoDataFromHead(ctx context.Context) error {
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

	photoURL, err := a.URL(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoURL, bytes.NewReader([]byte{})) //xxx consider seeing if I can pass a nil reader in all these spots I am passing empty bytes
	if err != nil {
		return err
	}
	req.Header.Add("Range", "bytes=0-0")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
		return err
	}

	if resp.StatusCode != http.StatusPartialContent {
		return errors.New(resp.Status)
	}

	contentRange := resp.Header.Get("Content-Range")
	matches := sizeFromContentRangeRegexp.FindStringSubmatch(contentRange)
	if len(matches) != 2 {
		return errors.New("could not parse Content-Range header")
	}
	sizeStr := matches[1]
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return err
	}

	a.size = size
	return nil
}
