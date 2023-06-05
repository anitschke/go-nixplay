package httpx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func DoUnmarshalJSONResponse(client Client, request *http.Request, response any) error {
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := StatusError(resp); err != nil {
		//xxx
		b, _ := io.ReadAll(resp.Body)
		fmt.Println(string(b))

		// io.Copy(io.Discard, resp.Body) //xxx
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, response)
}
