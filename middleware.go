package caddyesi

import (
	"net/http"
	"strconv"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/responseproxy"
	"github.com/corestoreio/errors"
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

	fullRespBuf := bufpool.Get()
	defer bufpool.Put(fullRespBuf)

	// responseproxy should be a pipe writer to avoid blowing up the memory by
	// writing everything into a buffer continue serving and gather the content
	// into a buffer for later analyses.
	code, err := mw.Next.ServeHTTP(responseproxy.WrapBuffered(fullRespBuf, w), r)
	if err != nil {
		return 0, err
	}

	pageID, esiEntities := cfg.ESITagsByRequest(r)
	if len(esiEntities) == 0 {
		// does the following code even work?

		// within this IF block we make sure with the Group.Do call that ESI
		// tags to a specific page get only parsed once even if multiple
		// requests are coming in to the same page. therefore make sure your
		// pageID has been calculated correctly.

		result, err, shared := mw.Group.Do(strconv.FormatUint(pageID, 10), func() (interface{}, error) {

			entities, err := esitag.Parse(fullRespBuf) // for now a NewReader, might be removed
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse",
					log.Err(err), log.Uint64("page_id", pageID), loghttp.Request("request", r),
					log.Stringer("response_content", fullRespBuf), // lots of data here ...
				)
			}
			if err != nil {
				return nil, errors.Wrapf(err, "[caddyesi] Grouped parsing failed ID %d", pageID)
			}
			entities.ApplyLogger(cfg.Log)
			cfg.StoreESITags(pageID, entities)

			return entities, nil
		})
		if err != nil {
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.Group.Do",
					log.Err(err), loghttp.Request("request", r), log.Stringer("config", cfg),
					log.Bool("shared", shared), log.Uint64("page_id", pageID),
				)
			}
			return http.StatusInternalServerError, err
		}
		var ok bool
		esiEntities, ok = result.(esitag.Entities)
		if !ok {
			return http.StatusInternalServerError, errors.NewFatalf("[caddyesi] A programmer made a terrible mistake: %#v", result)
		}
	}

	// trigger the DoRequests and query all backend resources in parallel
	tags, err := esiEntities.QueryResources(r)
	if err != nil {
		if err != nil {
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.esiEntities.QueryResources",
					log.Err(err), loghttp.Request("request", r), log.Stringer("config", cfg),
					log.Uint64("page_id", pageID),
				)
			}
			return http.StatusInternalServerError, err
		}
	}

	// fullRespBuf maybe the reader needs to be reset

	// replace the esi tags with the content from the resources
	// after finishing the parsing and replacing we dump the output to the client.
	if err := tags.InjectContent(fullRespBuf, w); err != nil {
		return http.StatusInternalServerError, err
	}

	return code, nil
}
