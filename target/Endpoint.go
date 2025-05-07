package target

import "net/http"

type Collection []Endpoint

// An endpoint is a hostname and a suitable http client
type Endpoint struct {
	Hostname string
	Client   *http.Client
}
