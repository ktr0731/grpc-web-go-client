package transport

import "net/http"

type ConnectOptions struct{
	// use this client instead of the default client if set
	Client *http.Client
}
