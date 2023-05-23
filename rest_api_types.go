package nixplay

import "github.com/anitschke/go-nixplay/httpx"

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

func (resp albumPhotosResponse) ToPhotos(album Container, authClient httpx.Client, client httpx.Client) []Photo {
	photos := make([]Photo, 0, len(resp.Photos))
	for _, p := range resp.Photos {
		photos = append(photos, p.ToPhoto(album, authClient, client))
	}
	return photos
}

type nixplayAlbumPhoto struct {
	FileName string  `json:"filename"`
	ID       uint64  `json:"id"`
	MD5      MD5Hash `json:"md5"`
	URL      string  `json:"url"`
}

func (p nixplayAlbumPhoto) ToPhoto(album Container, authClient httpx.Client, client httpx.Client) Photo {
	size := int64(-1)
	return newAlbumPhoto(album, authClient, client, p.FileName, p.MD5, p.ID, size, p.URL)
}

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
