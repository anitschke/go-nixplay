package httpx

import (
	"fmt"
	"io"
	"net/http"
)

// xxx doc
func StatusError(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http status: %s: body: %s", resp.Status, body)
	}
	return nil
}
