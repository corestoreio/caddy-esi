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

func (e ESI) configForPath(r *http.Request) *Config {
	for _, c := range e.rc.Configs {
		if httpserver.Path(r.URL.Path).Matches(c.PathScope) { // not negated
			return c
		}
	}
	return nil
}

// ServeHTTP implements the http.Handler interface.
func (e ESI) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	cfg := e.configForPath(r)
	if cfg == nil {
		return e.Next.ServeHTTP(w, r) // exit early
	}

	// todo: we must wrap the responseWrite to provide stream parsing and replacement other handlers
	// "github.com/zenazn/goji/web/mutil"
	// parse the stream ... build the cache of ESI tags.
	// requestID(r) uint64
	//lw := mutil.WrapWriter(w)

	// We only deal with HEAD/GET
	switch r.Method {
	case http.MethodGet, http.MethodHead:
	default:
		return http.StatusMethodNotAllowed, nil
	}

	return http.StatusTeapot, nil
}
