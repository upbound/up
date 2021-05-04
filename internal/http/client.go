package http

import "net/http"

var _ Client = &http.Client{}

// Client is an HTTP client.
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}
