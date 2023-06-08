package nixplay

import (
	"context"
	"fmt"
	"net/http"

	"github.com/anitschke/go-nixplay/httpx"
	"github.com/anitschke/go-nixplay/types"
)

const albumAddIDName = "albumId"

func newAlbum(client httpx.Client, name string, nixplayID uint64, photoCount int64) *container {
	return newContainer(client, types.AlbumContainerType, name, nixplayID, photoCount, albumPhotosPage, albumDeleteRequest, albumAddIDName)
}

func albumDeleteRequest(ctx context.Context, nixplayID uint64) (*http.Request, error) {
	url := fmt.Sprintf("https://api.nixplay.com/album/%d/delete/json/", nixplayID)
	return http.NewRequestWithContext(context.Background(), http.MethodPost, url, http.NoBody)
}

func albumPhotosPage(ctx context.Context, client httpx.Client, container Container, nixplayID uint64, page uint64, pageSize uint64) ([]Photo, error) {
	page++ // nixplay uses 1 based indexing for album pages but provided page assumes 0 based.

	limit := pageSize
	url := fmt.Sprintf("https://api.nixplay.com/album/%d/pictures/json/?page=%d&limit=%d", nixplayID, page, limit)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}

	var albumPhotos albumPhotosResponse
	if err := httpx.DoUnmarshalJSONResponse(client, req, &albumPhotos); err != nil {
		return nil, err
	}

	return albumPhotos.ToPhotos(container, client)
}
