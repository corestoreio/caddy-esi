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
	"net/http/httptest"
	"strconv"

	"github.com/SchumacherFM/caddyesi/esitag"
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

	// maybe use a hashing function to check if content changes ... ?

	//fullRespBuf := bufpool.Get()
	//defer bufpool.Put(fullRespBuf)

	// responseproxy should be a pipe writer to avoid blowing up the memory by
	// writing everything into a buffer. We could uses here one or two pipes.
	// One pipe only for InjectContent when the ESI entities are already
	// available and two pipes when we first must analyze the HTML page (pipe1)
	// and at the end InjectContent (pipe2).

	rec := httptest.NewRecorder()

	//code, err := mw.Next.ServeHTTP(responseproxy.WrapBuffered(fullRespBuf, w), r)
	code, err := mw.Next.ServeHTTP(rec, r)
	if !cfg.IsStatusCodeAllowed(code) || err != nil {
		return code, err
	}

	pageID, esiEntities := cfg.ESITagsByRequest(r)
	if esiEntities == nil {

		// Create a 2nd buffer just for reading in calculateESITags. this is stupid and must be refactored.
		bodyBuf := new(bytes.Buffer)
		*bodyBuf = *(rec.Body)

		esiEntities, err = mw.calculateESITags(pageID, bodyBuf, cfg)
		if err != nil {
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.calculateESITags",
					log.Err(err), log.Uint64("page_id", pageID), loghttp.Request("request", r), log.Stringer("config", cfg),
				)
			}
			return http.StatusInternalServerError, err
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

	//fmt.Printf("ResponseRecorder: %#v\n\n", rec.Result())

	// TODO(CyS) copy the headers and status code from rec into w, i think

	// Replace the esi tags with the content from the resources. After finishing
	// the parsing and replacing we dump the output to the client.
	if err := tags.InjectContent(rec.Body, w); err != nil {
		return http.StatusInternalServerError, err
	}

	return code, nil
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
