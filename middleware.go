package caddyesi

import (
	"bytes"
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

	fullRespBuf := bufpool.Get()
	defer bufpool.Put(fullRespBuf)

	// responseproxy should be a pipe writer to avoid blowing up the memory by
	// writing everything into a buffer. We could uses here one or two pipes.
	// One pipe only for InjectContent when the ESI entities are already
	// available and two pipes when we first must analyze the HTML page (pipe1)
	// and at the end InjectContent (pipe2).
	code, err := mw.Next.ServeHTTP(responseproxy.WrapBuffered(fullRespBuf, w), r)
	if !cfg.IsStatusCodeAllowed(code) || err != nil {
		return code, err
	}

	pageID, esiEntities := cfg.ESITagsByRequest(r)
	if len(esiEntities) == 0 {

		// within this IF block we make sure with the Group.Do call that ESI
		// tags to a specific page get only parsed once even if multiple
		// requests are coming in to the same page. therefore make sure your
		// pageID has been calculated correctly.

		result, err, shared := mw.Group.Do(strconv.FormatUint(pageID, 10), func() (interface{}, error) {

			entities, err := esitag.Parse(bytes.NewReader(fullRespBuf.Bytes())) // for now a NewReader, might be removed
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse",
					log.Err(err), log.Uint64("page_id", pageID), loghttp.Request("request", r),
					log.Stringer("response_content", fullRespBuf), // lots of data here ...
				)
			}
			if err != nil {
				return nil, errors.Wrapf(err, "[caddyesi] Grouped parsing failed ID %d", pageID)
			}
			cfg.UpsertESITags(pageID, entities)

			return entities, nil
		})
		if err != nil {
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.Group.Do.Error",
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

	// trigger the DoRequests and query all backend resources in parallel.
	// TODO(CyS) Coalesce requests
	tags, err := esiEntities.QueryResources(r)
	if err != nil {
		if err != nil {
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.esiEntities.QueryResources.Error",
					log.Err(err), loghttp.Request("request", r), log.Stringer("config", cfg),
					log.Uint64("page_id", pageID),
				)
			}
			return http.StatusInternalServerError, err
		}
	}

	// Replace the esi tags with the content from the resources. After finishing
	// the parsing and replacing we dump the output to the client.
	if err := tags.InjectContent(fullRespBuf, w); err != nil {
		return http.StatusInternalServerError, err
	}

	return code, nil
}
