package httpx

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func NewPostFormRequest(ctx context.Context, endpoint string, values url.Values) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create form request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}
