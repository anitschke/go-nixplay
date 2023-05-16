package httpx

import "net/http"

// Client is an interface for an http.Client from the standard library that
// allows us to more easily extend and/or mock out the existing http client from
// the standard library.
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}
