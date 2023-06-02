package nixplay

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
)

//xxx see what code can be shared between playlist photo and album photo

type playlistPhoto struct {
}

var playlistPhotoImpl = (photoImplementation)(playlistPhoto{})

func (playlistPhoto) DeleteRequest(ctx context.Context, scope DeleteScope, container Container, nixplayID string) (*http.Request, error) {
	playlist, ok := container.(*playlist)
	if !ok {
		return nil, fmt.Errorf("failed to cast container to playlist")
	}

	url := fmt.Sprintf("https://api.nixplay.com/v3/playlists/%d/items?id=%s", playlist.nixplayID, nixplayID)
	return http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte{}))
}
