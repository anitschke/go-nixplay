package nixplay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/anitschke/go-nixplay/httpx"
)

// This regexp will parse a content range to give us the full size of the file
// in the range request. It isn't fully compliant with parsing RFC 7233 but the
// other cases for the content range header specified by RFC 7233 don't provide
// the length so I think this is ok for our use case. See
// https://datatracker.ietf.org/doc/html/rfc7233#section-4.2
var sizeFromContentRangeRegexp = regexp.MustCompile(`^bytes \d+-\d+/(\d+)$`)

// This regexp will parse the path portion of a photo URL and give us the MD5
// hash of the file so we can get the hash without needing to download the
// entire file and hashing it. Note that this regex depends on the fact the that
// photo url happens to contain the file's MD5 hash as part of the URL. For
// example
//
// URL: https://nixplay-prod-original.s3.us-west-2.amazonaws.com/3293355/3293355_073089b1d67a56c63b989d4e5f660ab8.jpg?AWSAccessKeyId=REDACTED&Expires=REDACTED&Signature=REDACTED
// MD5: 073089b1d67a56c63b989d4e5f660ab8
//
// However rather than parse the entire URL with the regexp we will use
// url.Parse to parse the URL and then just use the regexp to parse the path of
// the url. ie "/3293355/3293355_073089b1d67a56c63b989d4e5f660ab8.jpg"
var md5HashFromPhotoURLPath = regexp.MustCompile(`^/\d+/\d+_([A-Fa-f0-9]{32}).jpg$`)

// photo is the type that implements the Photo interface.
//
// The object hierarchy here gets a little strange because there are some
// differences between album photos and playlist photos, but 90% of the code is
// the same. So photo does most of the heavy lifting and then makes a call out
// to photoImplementation when it needs implementation specific info regarding
// album/playlist photos.
type photo struct {
	name    string
	id      ID
	md5Hash MD5Hash

	container  Container
	authClient httpx.Client
	client     httpx.Client

	//xxx needs mutex for things that can be updated

	nixplayID string //xxx change to uint64
	size      int64
	url       string
}

func newPhoto(container Container, authClient httpx.Client, client httpx.Client, name string, md5Hash *MD5Hash, nixplayID string, size int64, url string) (*photo, error) {
	// Based on current usage of newPhoto the MD5 hash should always be able to
	// be provided, either because we are uploading a photo so we can do the
	// hash ourselves, or because we are getting a list of photos and can
	// provided the MD5 Hash directly (in the case of album photos) extract the
	// MD5 hash from the URL (in the case of playlist photos). For now we will
	// error if one of these is not provided. In the future things can always be
	// updated so we can get the md5hash on demand by getting the url, but lets
	// keep the code simple for now.
	if md5Hash == nil {
		if url == "" {
			return nil, errors.New("MD5 or photo URL must be provided")
		}
		md5HashValue, err := md5HashFromPhotoURL(url)
		if err != nil {
			return nil, err
		}
		md5Hash = &md5HashValue
	}

	// Unfortunately when we upload a photo there isn't any way to get the
	// nixplay ID of the photo without getting ALL the photos in that
	// album/playlist and searching through them all. But ideally we want some
	// sort of identifier for an image that is stable without having to do this.
	//
	// Nixplay allows duplicate named pictures int he same album but does not
	// allow two copies of the same picture. So what we will do is compute the
	// ID to be the hash album ID + MD5Hash of the image. This lets us have a
	// stable ID for the image even if we don't know what Nixplay's internal
	// identifier is.
	//
	// Nixplay allows duplicate named pictures in the same playlist, it will
	// even allow adding the same photo from an album to a playlist multiple
	// times and the photo will show up in the playlist multiple times. However
	// this behavior is a little inconsistent. If I try to directly upload the
	// same photo to the playlist multiple times it only shows up once, BUT it
	// does allow my to add a photo from an album multiple times.
	//
	// This unfortunately means that if we want to support this functionally we
	// can't come up with an ID that works. So I think for now we will just say
	// that this library doesn't support dealing with duplicate photos in the
	// same playlist, or at least doesn't guarantee that these photos will have
	// a unique ID.
	//
	// So with all that being said we will hash the container id together with
	// the MD5 hash of the photo and that should give us a unique
	// enough ID with the exception of the above mentioned issue.

	//xxx document the above incompatibility somewhere in reademe

	containerID := container.ID()
	hasher := sha256.New()
	hasher.Write(containerID[:]) // shouldn't ever error so we don't need to check for one
	hasher.Write(md5Hash[:])
	id := ID(hasher.Sum([]byte{}))

	return &photo{
		name:    name,
		id:      id,
		md5Hash: *md5Hash,

		container:  container,
		authClient: authClient,
		client:     client,

		nixplayID: nixplayID,
		size:      size,
		url:       url,
	}, nil
}

var _ = (Photo)((*photo)(nil))

func md5HashFromPhotoURL(photoURL string) (returnHash MD5Hash, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to parse playlist photo URL for MD5 hash %q: %w", photoURL, err)
		}
	}()

	urlObj, err := url.Parse(photoURL)
	if err != nil {
		return MD5Hash{}, err
	}

	matches := md5HashFromPhotoURLPath.FindStringSubmatch(urlObj.Path)
	if len(matches) != 2 {
		return MD5Hash{}, errors.New("regexp failed to find MD5 hash in URL")
	}
	hashStr := matches[1]
	var hash MD5Hash
	err = hash.UnmarshalText([]byte(hashStr))
	if err != nil {
		return MD5Hash{}, err
	}
	return hash, nil
}

func (p *photo) Name(ctx context.Context) (string, error) {
	if p.name == "" {
		if err := p.populatePhotoDataFromPictureEndpoint(ctx); err != nil {
			return "", err
		}
	}
	if p.name == "" {
		return "", errors.New("failed to determine photo name")
	}

	return p.name, nil
}

func (p *photo) ID() ID {
	return p.id
}

func (p *photo) Size(ctx context.Context) (int64, error) {
	if p.size == -1 {
		err := p.populatePhotoDataFromHead(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to get image size: %w", err)
		}
	}
	if p.size == -1 {
		return 0, errors.New("unable to determine photo size")
	}

	return p.size, nil
}

func (p *photo) MD5Hash(ctx context.Context) (MD5Hash, error) {
	return p.md5Hash, nil
}

func (p *photo) URL(ctx context.Context) (string, error) {
	if p.url == "" {
		if err := p.populatePhotoDataFromListSearch(ctx); err != nil {
			return "", fmt.Errorf("failed to get image url: %w", err)
		}
	}
	if p.url == "" {
		return "", errors.New("unable to determine photo URL")
	}
	return p.url, nil
}

func (p *photo) Open(ctx context.Context) (retReadCloser io.ReadCloser, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to open photo: %w", err)
		}
	}()

	photoURL, err := p.URL(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoURL, bytes.NewReader([]byte{})) //xxx consider seeing if I can pass a nil reader in all these spots I am passing empty bytes
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
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

	if p.size == -1 {
		sizeStr := resp.Header.Get("Content-Length")
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			return nil, err
		}
		p.size = size
	}

	return resp.Body, nil
}

func (p *photo) Delete(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to delete photo: %w", err)
		}
	}()

	nixplayID, err := p.getNixplayID(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.nixplay.com/picture/%s/delete/json/", nixplayID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}
	resp, err := p.authClient.Do(req)
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

	return p.container.onPhotoDelete(ctx, p)
}

func (p *photo) getNixplayID(ctx context.Context) (string, error) {
	if p.nixplayID == "" {
		if err := p.populatePhotoDataFromListSearch(ctx); err != nil {
			return "", fmt.Errorf("failed to get internal Nixplay ID: %w", err)
		}
	}
	if p.nixplayID == "" {
		return "", errors.New("unable to determine internal Nixplay ID")
	}
	return p.nixplayID, nil
}

func (p *photo) populatePhotoDataFromListSearch(ctx context.Context) error {
	// Unfortunately when we add a new photo there doesn't seem to be any API to
	// get nixplay's ID or URL for the photo. So what we need to do is query the
	// parent album for all it's photos and then search for this photo by
	// looking for one that has the same md5hash since we can compute and store
	// that when we are doing the upload. (we can't match by name because
	// nixplay allows multiple files with the same name.)
	//
	// Containers to have an internal cache of photos. So the first time we try
	// to get the data we may get lucky and might already have the data from a
	// previous update of the container's cache.
	//
	// But that data we want may not be in the cache, so if the data isn't there
	// the first time around we need to reset the cache to force it to
	// repopulate.

	// xxx xxx LEFT OF HERE xxx xxx xxxxxx
	//
	// This call to Photos can't use the cache all the time because we need to
	// get the full data from API request so we can populate the data we don't
	// know. The easy fix for now would be to just always invalidate the cache
	// when adding a new photo, but I don't know if I want to go down that path.
	// Actually that might be the best option anyway since it will be easiest to
	// code and I don't thing that big of a performance overhead.

	found, err := p.attemptPopulatePhotoDataFromListSearch(ctx)
	if err != nil {
		return err
	}
	if found {
		// Fast path! we were able to get the data first try, probably from the cache
		return nil
	}

	// Slow path :( invalidate the cache and try again which will repopulate by querying nixplay
	p.container.ResetCache()

	found, err = p.attemptPopulatePhotoDataFromListSearch(ctx)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("incomplete photo data in list")
	}
	return nil
}

func (p *photo) attemptPopulatePhotoDataFromListSearch(ctx context.Context) (bool, error) {
	pFromContainer, err := p.container.PhotoWithID(ctx, p.ID())
	if err != nil {
		return false, err
	}
	if pFromContainer != nil {
		pp, ok := pFromContainer.(*photo) //xxx should I add an API a.album.photos that gives it to me as photos so I don't need to cast?
		if !ok {
			panic("failed to cast to *photo in populatePhotoDataFromListSearch")
		}

		if pp.nixplayID != "" && pp.url != "" {
			p.nixplayID = pp.nixplayID
			p.url = pp.url
			return true, nil
		}
	}

	// Couldn't find the data
	return false, nil
}

func (p *photo) populatePhotoDataFromPictureEndpoint(ctx context.Context) error {
	id, err := p.getNixplayID(ctx)
	if err != nil {
		return err
	}

	idAsInt, err := strconv.Atoi(id)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.nixplay.com/picture/%d/", idAsInt)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewReader(nil)) //xxx consider seeing if I can pass a nil reader in all these spots I am passing empty bytes
	if err != nil {
		return err
	}

	var nixplayPhoto nixplayAlbumPhoto
	if err := httpx.DoUnmarshalJSONResponse(p.authClient, req, &nixplayPhoto); err != nil {
		return err
	}

	photoFromPicEndpoint, err := nixplayPhoto.ToPhoto(p.container, p.authClient, p.client)
	if err != nil {
		return err
	}

	p.name, err = photoFromPicEndpoint.Name(ctx)
	return err
}

func (p *photo) populatePhotoDataFromHead(ctx context.Context) error {
	// xxx doc Getting the size of the photo is a little tricky. Ideally we could
	// use the HEAD method but the way s3 Signature works is it is for a
	// specific method. xxx add rest of details
	//
	// https://stackoverflow.com/a/39663152 curl -v -r 0-0
	//
	// This relies on s3 honoring our request for only a single byte which at
	// the moment it does so I think we can just assume it will continue to do
	// so and not complicate the code more by trying to make it handle future
	// fringe cases where s3 doesn't do what we are expecting.

	photoURL, err := p.URL(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, photoURL, bytes.NewReader(nil)) //xxx consider seeing if I can pass a nil reader in all these spots I am passing empty bytes
	if err != nil {
		return err
	}
	req.Header.Add("Range", "bytes=0-0")

	resp, err := p.client.Do(req)
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
		return fmt.Errorf("could not parse Content-Range header %q", contentRange)
	}
	sizeStr := matches[1]
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return err
	}

	p.size = size
	return nil
}