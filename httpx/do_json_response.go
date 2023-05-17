package httpx

import (
	"encoding/json"
	"io"
	"net/http"
)

func DoUnmarshalJSONResponse(client Client, request *http.Request, response any) error {
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	//xxx check response code

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, response)
}
