package nixplay

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
