// Copyright 2016-2017, Cyrill @ Schumacher.fm and the CaddyESI Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package caddyesi

import (
	"bytes"
	"io"
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
	// maybe use a hashing function to check if content changes ... or another
	// endpoint to clear the internal cache ESI tags ?

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

	pageID, esiEntities := cfg.ESITagsByRequest(r)
	if esiEntities == nil {
		// Slow path because ESI cache tag is empty and we need to analyse the buffer.

		buf := bufpool.Get()
		defer bufpool.Put(buf)

		bufResW := responseproxy.WrapBuffered(buf, w)

		// We must wait until every single byte has been written into the buffer.
		code, err := mw.Next.ServeHTTP(bufResW, r)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		if !cfg.IsStatusCodeAllowed(code) {
			// Headers and Status Code already written to the client but we need
			// to flush the real content!
			// TODO(CyS) make an entry in the ESI tag map that subsequent non-allowed requests to the same resource can be skipped
			if _, err := w.Write(buf.Bytes()); err != nil {
				return http.StatusInternalServerError, err
			}
			return code, nil
		}

		bufRdr := bytes.NewReader(buf.Bytes())

		// Lets parse the buffer to find ESI tags. First Read
		esiEntities, err = mw.calculateESITags(pageID, bufRdr, cfg)
		if err != nil {
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.calculateESITags",
					log.Err(err), log.Uint64("page_id", pageID), loghttp.Request("request", r), log.Stringer("config", cfg),
				)
			}
			return http.StatusInternalServerError, err
		}

		// Trigger the queries to the resource backends in parallel
		// TODO(CyS) Coalesce requests
		tags, err := esiEntities.QueryResources(r)

		if err != nil {
			// wrong behaviour compared to below
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.esiEntities.QueryResources.Error",
					log.Err(err), loghttp.Request("request", r), log.Stringer("config", cfg),
					log.Uint64("page_id", pageID),
				)
			}
			return http.StatusInternalServerError, err
		}

		if _, err := bufRdr.Seek(0, 0); err != nil {
			return http.StatusInternalServerError, err
		}

		// read the 2nd time from the buffer to finally inject the content from the resource backends
		// into the HTML page
		if _, err := tags.InjectContent(bufRdr, w); err != nil {
			return http.StatusInternalServerError, err
		}

		return code, err
	}

	////////////////////////////////////////////////////////////////////////////////
	// Proceed from cache

	chanTags := make(chan esitag.DataTags)
	go func() {
		// trigger the DoRequests and query all backend resources in parallel.
		// TODO(CyS) Coalesce requests
		tags, err := esiEntities.QueryResources(r)
		if err != nil {
			// todo better error handling and propagation

			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.esiEntities.QueryResources.Error",
					log.Err(err), loghttp.Request("request", r), log.Stringer("config", cfg),
					log.Uint64("page_id", pageID),
				)
			}
		}
		chanTags <- tags
		close(chanTags)
	}()

	wpResW := responseproxy.WrapPiped( // this one would panic because error in ppr.CloseWithError
		esitag.NewDataTagsInjector(chanTags, w), // runs in a goroutine
		w,
	)
	defer func() {
		if err := wpResW.Close(); err != nil {
			panic(err) // only now during dev
		}
	}()

	// Start Serving and writing into the pipe
	code, err := mw.Next.ServeHTTP(wpResW, r)
	if err != nil {
		// discard the cErr channel to the GC ... hope that works
		return http.StatusInternalServerError, errors.Wrap(err, "[caddyesi] Error from Next.ServeHTTP")
	}
	if !cfg.IsStatusCodeAllowed(code) {
		// too late because the goroutine injectcontent has already written the data
		cfg.skipped = true
		// todo test it

	}

	return code, err

}

func (mw *Middleware) calculateESITags(pageID uint64, body io.Reader, cfg *PathConfig) (esitag.Entities, error) {
	// within this IF block we make sure with the Group.Do call that ESI
	// tags to a specific page get only parsed once even if multiple
	// requests are coming in to the same page. therefore make sure your
	// pageID has been calculated correctly.

	// run a performance load test to see if it's worth to switch to Group.DoChan
	result, err, shared := mw.Group.Do(strconv.FormatUint(pageID, 10), func() (interface{}, error) {

		var bodyBuf *bytes.Buffer
		if cfg.Log.IsDebug() {
			bodyBuf = new(bytes.Buffer)
			body = io.TeeReader(body, bodyBuf)
		}

		entities, err := esitag.Parse(body)
		if cfg.Log.IsDebug() {
			cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse",
				log.Err(err), log.Uint64("page_id", pageID), log.Int("tag_count", len(entities)), log.Stringer("content", bodyBuf),
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
				log.Err(err), log.Stringer("config", cfg),
				log.Bool("shared", shared), log.Uint64("page_id", pageID),
			)
		}
		return nil, err
	}

	esiEntities, ok := result.(esitag.Entities)
	if !ok {
		return nil, errors.NewFatalf("[caddyesi] A programmer made a terrible mistake: %#v", result)
	}
	return esiEntities, nil
}
