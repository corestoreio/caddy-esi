package caddyesi

import (
	"bytes"
	"log"
	"net/http"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/responseproxy"
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
		log.Println("[DEBUG] ESI Not config found") // only during dev
		return mw.Next.ServeHTTP(w, r)              // exit early
	}
	if !cfg.IsRequestAllowed(r) {
		log.Println("[DEBUG] ESI request not allowed") // only during dev
		return mw.Next.ServeHTTP(w, r)                 // go on ...
	}

	// maybe use a hashing function to check if content changes ...

	// todo: we must wrap the ResponseWriter to provide stream parsing and replacement other handlers
	// parse the stream ... build the cache of ESI tags.

	buf := bufpool.Get()
	defer bufpool.Put(buf)

	wBuf := responseproxy.WrapBuffered(buf, w) // should be a pipe writer to avoid blowing up the memory

	// continue serving and gather the content into a buffer for later analyses.
	code, err := mw.Next.ServeHTTP(wBuf, r)
	if err != nil {
		return 0, err
	}

	requestID, tags := cfg.ESITagsByRequest(r)
	if len(tags) == 0 {
		var err2 error
		tags, err2 = esitag.Parse(bytes.NewReader(buf.Bytes())) // for now a NewReader, might be removed
		if err2 != nil {
			return http.StatusInternalServerError, err2
		}

		log.Printf("[DEBUG] ESI requestID %d", requestID)
		cfg.StoreESITags(requestID, tags)
	}

	// now we have here our parsed ESI tags ...
	println(tags.String())

	// after finishing the parsing and replacing we dump the output to the client.
	if _, err := w.Write(buf.Bytes()); err != nil {
		return 0, err
	}

	return code, nil
}
