package nixplay

import (
	"context"
	"fmt"
	"net/http"

	"github.com/anitschke/go-nixplay/httpx"
	"github.com/anitschke/go-nixplay/types"
)

const playlistAddIDName = "playlistId"

func newPlaylist(client httpx.Client, name string, nixplayID uint64, photoCount int64) *container {
	return newContainer(client, types.PlaylistContainerType, name, nixplayID, photoCount, playlistPhotosPage, playlistDeleteRequest, playlistAddIDName)
}

func playlistDeleteRequest(ctx context.Context, nixplayID uint64) (*http.Request, error) {
	url := fmt.Sprintf("https://api.nixplay.com/v3/playlists/%d", nixplayID)
	return http.NewRequestWithContext(context.Background(), http.MethodDelete, url, http.NoBody)
}

func playlistPhotosPage(ctx context.Context, client httpx.Client, container Container, nixplayID uint64, page uint64, pageSize uint64) ([]Photo, error) {
	limit := pageSize
	offset := page * limit
	url := fmt.Sprintf("https://api.nixplay.com/v3/playlists/%d/slides?size=%d&offset=%d", nixplayID, limit, offset)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	var playlistPhotos playlistPhotosResponse
	if err := httpx.DoUnmarshalJSONResponse(client, req, &playlistPhotos); err != nil {
		return nil, err
	}

	return playlistPhotos.ToPhotos(container, client)
}
