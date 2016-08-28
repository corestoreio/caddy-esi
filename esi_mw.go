package esi

import (
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// ESI implements the ESI tag middleware
type ESI struct {
	// Root the Server root
	Root string

	//FileSys  jails the requests to site root with a mock file system
	FileSys http.FileSystem

	// Next HTTP handler in the chain
	Next httpserver.Handler

	// The list of ESI configurations
	rc *RootConfig
}

// ServeHTTP implements the http.Handler interface.
func (e ESI) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	cfg := e.rc.PathConfigs.ConfigForPath(r)
	if cfg == nil {
		return e.Next.ServeHTTP(w, r) // exit early
	}

	// todo: we must wrap the ResponseWriter to provide stream parsing and replacement other handlers
	// parse the stream ... build the cache of ESI tags.

	// We only deal with HEAD/GET
	switch r.Method {
	case http.MethodGet, http.MethodHead:
	default:
		return e.Next.ServeHTTP(w, r) // go on ...
	}

	tags := e.rc.ESITagsByRequest(r)
	if len(tags) == 0 {
		// no tags found
		// start parsing the stream; wrap the ResponseWriter
	}

	return e.Next.ServeHTTP(w, r)
}
