package nixplay

// This file contains types to support unmarshalling all of the responses we get
// back from Nixplay

type albumsResponse []album

func (albums albumsResponse) ToContainers() []Container {
	containers := make([]Container, 0, len(albums))
	for _, a := range albums {
		containers = append(containers, a.ToContainer())
	}
	return containers
}

type album struct {
	PhotoCount uint64 `json:"photo_count"`
	Title      string `json:"title"`
	ID         uint64 `json:"id"`
}

func (a album) ToContainer() Container {
	return Container{
		ContainerType: AlbumContainerType,
		Name:          a.Title,
		ID:            a.ID,
		PhotoCount:    a.PhotoCount,
	}
}

type playlistResponse []playlist

func (playlists playlistResponse) ToContainers() []Container {
	containers := make([]Container, 0, len(playlists))
	for _, p := range playlists {
		containers = append(containers, p.ToContainer())
	}
	return containers
}

type playlist struct {
	PictureCount uint64 `json:"picture_count"`
	Name         string `json:"name"`
	ID           uint64 `json:"id"`
}

func (p playlist) ToContainer() Container {
	return Container{
		ContainerType: PlaylistContainerType,
		Name:          p.Name,
		ID:            p.ID,
		PhotoCount:    p.PictureCount,
	}
}

type createPlaylistRequest struct {
	Name string `json:"name"`
}

type createPlaylistResponse struct {
	PlaylistId uint64 `json:"playlistId"`
}

type albumPhotosResponse struct {
	Photos []albumPhoto `json:"photos"`
}

func (resp albumPhotosResponse) ToPhotos() []Photo {
	photos := make([]Photo, 0, len(resp.Photos))
	for _, p := range resp.Photos {
		photos = append(photos, p.ToPhoto())
	}
	return photos
}

type albumPhoto struct {
	FileName string  `json:"filename"`
	ID       uint64  `json:"id"`
	MD5      MD5Hash `json:"md5"`
	URL      string  `json:"url"`
}

func (p albumPhoto) ToPhoto() Photo {
	return Photo{
		Name:                p.FileName,
		ID:                  p.ID,
		MD5Hash:             p.MD5,
		URL:                 p.URL,
		parentContainerType: AlbumContainerType,
	}

	// xxx Photo also has a Size proprety to have the file size but it looks
	// like nixplay doesn't give that to me. Look into if there is some way to
	// get this data since it might be require to get rclone to work.
	//
	// It looks like s3 has a way to get the size of an object without actually
	// downloading. So using s3 APIs is probably the best bet.
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
