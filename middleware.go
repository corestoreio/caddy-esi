package caddyesi

import (
	"bytes"
	"net/http"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/responseproxy"
	"github.com/corestoreio/log"
	loghttp "github.com/corestoreio/log/http"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"golang.org/x/sync/singleflight"
)

// Middleware implements the ESI tag middleware
type Middleware struct {
	Group singleflight.Group
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

func (mw *Middleware) selectTags(r *http.Request) {} // wtf?

// ServeHTTP implements the http.Handler interface.
func (mw *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) (int, error) {
	cfg := mw.PathConfigs.ConfigForPath(r)
	if cfg == nil {
		return mw.Next.ServeHTTP(w, r) // exit early
	}
	if !cfg.IsRequestAllowed(r) {
		if cfg.Log.IsDebug() {
			cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.IsRequestAllowed",
				log.Bool("allowed", false), loghttp.Request("request", r), log.Stringer("config", cfg),
			)
		}
		return mw.Next.ServeHTTP(w, r) // go on ...
	}

	// maybe use a hashing function to check if content changes ...

	// todo: we must wrap the ResponseWriter to provide stream parsing and replacement other handlers
	// parse the stream ... build the cache of ESI tags.

	pageIDStr := cfg.PageID(r)
	mw.Group.Do(pageIDStr, func() (interface{}, error) {

		return nil, nil
	})

	buf := bufpool.Get()
	defer bufpool.Put(buf)

	// responseproxy should be a pipe writer to avoid blowing up the memory by
	// writing everything into a buffer continue serving and gather the content
	// into a buffer for later analyses.
	code, err := mw.Next.ServeHTTP(responseproxy.WrapBuffered(buf, w), r)
	if err != nil {
		return 0, err
	}

	pageID, tags := cfg.ESITagsByRequest(r)
	if len(tags) == 0 {

		var err2 error
		tags, err2 = esitag.Parse(bytes.NewReader(buf.Bytes())) // for now a NewReader, might be removed
		if err2 != nil {
			return http.StatusInternalServerError, err2
		}

		if cfg.Log.IsDebug() {
			cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse",
				log.Uint64("page_id", pageID), loghttp.Request("request", r),
			)
		}
		cfg.StoreESITags(pageID, tags)
	}

	// now we have here our parsed ESI tags ...
	println(tags.String())

	// after finishing the parsing and replacing we dump the output to the client.
	if _, err := w.Write(buf.Bytes()); err != nil {
		return 0, err
	}

	return code, nil
}
