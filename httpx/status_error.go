package httpx

import (
	"fmt"
	"net/http"
)

func StatusError(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http status: %s", resp.Status)
	}
	return nil
}
