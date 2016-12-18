package caddyesi

import (
	"net/http"

	"github.com/mholt/caddy/caddyhttp/httpserver"
)

// Middleware implements the ESI tag middleware
type Middleware struct {
	// Root the Server root
	Root string

	//FileSys  jails the requests to site root with a mock file system
	FileSys http.FileSystem

	// Next HTTP handler in the chain
	Next httpserver.Handler

	// PathConfigs The list of ESI configurations for each path prefix and theirs
	// caches.
	PathConfigs
}

// ServeHTTP implements the http.Handler interface.
func (mw Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	cfg := mw.PathConfigs.ConfigForPath(r)
	if cfg == nil {
		return mw.Next.ServeHTTP(w, r) // exit early
	}
	if !cfg.isRequestAllowed(r) {
		return mw.Next.ServeHTTP(w, r) // go on ...
	}

	// What's next?
	// - Calculate a unique identifier for each page. This ID will be used to
	//   look up the already parsed ESI tags to avoid re-parsing. See func requestID()
	// -

	// maybe use a hashing function to check if content changes ...

	// todo: we must wrap the ResponseWriter to provide stream parsing and replacement other handlers
	// parse the stream ... build the cache of ESI tags.

	tags := cfg.ESITagsByRequest(r)
	if len(tags) == 0 {
		// no tags found
		// start parsing the stream; wrap the ResponseWriter
	}

	// start replacement

	return mw.Next.ServeHTTP(w, r)
}
