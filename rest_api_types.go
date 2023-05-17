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
