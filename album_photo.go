package nixplay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
)

var errGlobalDeleteScopeNotForAlbums = errors.New("global delete scope not currently supported for albums")

type albumPhoto struct {
}

var albumPhotoImpl = (photoImplementation)(albumPhoto{})

func (albumPhoto) DeleteRequest(ctx context.Context, scope DeleteScope, container Container, nixplayID string) (*http.Request, error) {
	if scope == GlobalDeleteScope {
		return nil, errGlobalDeleteScopeNotForAlbums
	}

	url := fmt.Sprintf("https://api.nixplay.com/picture/%s/delete/json/", nixplayID)
	return http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte{}))
}
