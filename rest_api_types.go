package nixplay

import (
	"strconv"

	"github.com/anitschke/go-nixplay/httpx"
)

// This file contains types to support unmarshalling all of the responses we get
// back from Nixplay

type albumsResponse []nixplayAlbum

func (albums albumsResponse) ToContainers(authClient httpx.Client, client httpx.Client) []Container {
	containers := make([]Container, 0, len(albums))
	for _, a := range albums {
		containers = append(containers, a.ToContainer(authClient, client))
	}
	return containers
}

type nixplayAlbum struct {
	PhotoCount int64  `json:"photo_count"`
	Title      string `json:"title"`
	ID         uint64 `json:"id"`
}

func (a nixplayAlbum) ToContainer(authClient httpx.Client, client httpx.Client) Container {
	return newAlbum(authClient, client, a.Title, a.ID, a.PhotoCount)
}

type playlistsResponse []playlistResponse

func (playlists playlistsResponse) ToContainers(authClient httpx.Client, client httpx.Client) []Container {
	containers := make([]Container, 0, len(playlists))
	for _, p := range playlists {
		containers = append(containers, p.ToContainer(authClient, client))
	}
	return containers
}

type playlistResponse struct {
	PictureCount int64  `json:"picture_count"`
	Name         string `json:"name"`
	ID           uint64 `json:"id"`
}

func (p playlistResponse) ToContainer(authClient httpx.Client, client httpx.Client) Container {
	return newPlaylist(authClient, client, p.Name, p.ID, p.PictureCount)
}

type createPlaylistRequest struct {
	Name string `json:"name"`
}

type createPlaylistResponse struct {
	PlaylistId uint64 `json:"playlistId"`
}

type albumPhotosResponse struct {
	Photos []nixplayAlbumPhoto `json:"photos"`
}

func (resp albumPhotosResponse) ToPhotos(album Container, authClient httpx.Client, client httpx.Client) ([]Photo, error) {
	photos := make([]Photo, 0, len(resp.Photos))
	for _, p := range resp.Photos {
		asPhoto, err := p.ToPhoto(album, authClient, client)
		if err != nil {
			return nil, err
		}
		photos = append(photos, asPhoto)
	}
	return photos, nil
}

type nixplayAlbumPhoto struct {
	FileName string  `json:"filename"`
	ID       int     `json:"id"`
	MD5      MD5Hash `json:"md5"`
	URL      string  `json:"url"`
}

func (p nixplayAlbumPhoto) ToPhoto(album Container, authClient httpx.Client, client httpx.Client) (Photo, error) {
	size := int64(-1)
	return newPhoto(albumPhotoImpl, album, authClient, client, p.FileName, &p.MD5, strconv.Itoa(p.ID), size, p.URL)
}

type playlistPhotosResponse struct {
	Photos []nixplayPlaylistPhoto `json:"slides"`
}

func (resp playlistPhotosResponse) ToPhotos(album Container, authClient httpx.Client, client httpx.Client) ([]Photo, error) {
	photos := make([]Photo, 0, len(resp.Photos))
	for _, p := range resp.Photos {
		asPhoto, err := p.ToPhoto(album, authClient, client)
		if err != nil {
			return nil, err
		}
		photos = append(photos, asPhoto)
	}
	return photos, nil
}

type nixplayPlaylistPhoto struct {
	FileName       string `json:"filename"`
	PlaylistItemID string `json:"playlistItemId"`
	URL            string `json:"originalUrl"`
}

func (p nixplayPlaylistPhoto) ToPhoto(album Container, authClient httpx.Client, client httpx.Client) (Photo, error) {
	var md5Hash *MD5Hash
	size := int64(-1)
	return newPhoto(playlistPhotoImpl, album, authClient, client, p.FileName, md5Hash, p.PlaylistItemID, size, p.URL)
}

//xxx need to extract md5 hash from file URL
//
// ex:
// MD5: 073089b1d67a56c63b989d4e5f660ab8
// URL: "https://nixplay-prod-original.s3.us-west-2.amazonaws.com/3293355/3293355_073089b1d67a56c63b989d4e5f660ab8.jpg?AWSAccessKeyId=AKIATMO6HVTTPMX3NF7V&Expires=1685577599&Signature=ap5jRu1%2BNYefl4iIHA0Pj5Av91w%3D"

type uploadTokenResponse struct {
	Token string `json:"token"`
}

// xxx remove stuff you don't need
type uploadNixplayResponseContainer struct {
	Data uploadNixplayResponse `json:"data"`
}

// xxx remove unused stuff
type uploadNixplayResponse struct {
	ACL            string `json:"acl"`
	Key            string `json:"key"`
	AWSAccessKeyID string `json:"AWSAccessKeyId"`
	Policy         string `json:"Policy"`
	Signature      string `json:"Signature"`
	// UserUploadID   string   `json:"userUploadId"`
	BatchUploadID string   `json:"batchUploadId"`
	UserUploadIDs []string `json:"userUploadIds"`
	FileType      string   `json:"fileType"`
	// FileSize       int      `json:"fileSize"`
	S3UploadURL string `json:"s3UploadUrl"`
}
