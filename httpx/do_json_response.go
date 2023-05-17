package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func DoUnmarshalJSONResponse(client Client, request *http.Request, response any) error {
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, response)
}
