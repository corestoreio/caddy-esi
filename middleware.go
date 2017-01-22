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
				log.Bool("benchIsResponseAllowed", false), loghttp.Request("request", r), log.Stringer("config", cfg),
			)
		}
		return mw.Next.ServeHTTP(w, r) // go on ...
	}
	if err := handleHeaderCommands(cfg, r); err != nil {
		// clears the ESI tags
		return http.StatusInternalServerError, err
	}

	pageID, esiEntities := cfg.ESITagsByRequest(r)
	if esiEntities == nil {
		// Slow path because ESI cache tag is empty and we need to analyse the buffer.
		return mw.serveBuffered(cfg, pageID, w, r)
	}

	////////////////////////////////////////////////////////////////////////////////
	// Proceed from map, filled with the parsed ESI tags.

	chanTags := make(chan esitag.DataTags, 1)
	defer close(chanTags)

	if len(esiEntities) > 0 {
		go func() {
			// TODO(CyS) Coalesce requests
			// trigger the DoRequests and query all backend resources in
			// parallel. Errors do are mostly of cancelled client requests which
			// the context propagates.
			tags, err := esiEntities.QueryResources(r)
			if err != nil {
				if cfg.Log.IsInfo() {
					cfg.Log.Info("caddyesi.Middleware.ServeHTTP.esiEntities.QueryResources.Error",
						log.Err(err), loghttp.Request("request", r), log.Stringer("config", cfg),
						log.Uint64("page_id", pageID),
					)
				}
			}
			chanTags <- tags
		}()
	} else {
		// We're not running in a goroutine so the channel must be buffered
		chanTags <- esitag.DataTags{}
	}

	return mw.Next.ServeHTTP(responseWrapInjector(chanTags, w), r)
}

// serveBuffered creates a http.ResponseWriter buffer, calls the next handler,
// waits until the buffer has been filled, parses the buffer for ESI tags,
// queries the resources and injects the data from the resources into the output
// towards the http.ResponseWriter.Write.
func (mw *Middleware) serveBuffered(cfg *PathConfig, pageID uint64, w http.ResponseWriter, r *http.Request) (int, error) {

	buf := bufpool.Get()
	defer bufpool.Put(buf)

	bufResW := responseWrapBuffer(buf, w)

	// We must wait until every single byte has been written into the buffer.
	code, err := mw.Next.ServeHTTP(bufResW, r)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Only plain text response is benchIsResponseAllowed, so detect content type
	if !isResponseAllowed(buf.Bytes()) {
		bufResW.FlushHeader(0)
		if _, err := w.Write(buf.Bytes()); err != nil {
			return http.StatusInternalServerError, err
		}
		return code, nil
	}

	bufRdr := bytes.NewReader(buf.Bytes())

	// Parse the buffer to find ESI tags. First buffer Read happens within this
	// Group.Do block. We make sure with the Group.Do call that ESI tags for a
	// specific page ID gets only parsed once, even if multiple requests are
	// coming in to for same page. Therefore you should make sure that your
	// pageID has been calculated correctly.

	// run a performance load test to see if it's worth to switch to Group.DoChan
	groupEntitiesResult, err, shared := mw.Group.Do(strconv.FormatUint(pageID, 10), func() (interface{}, error) {

		var body io.Reader = bufRdr
		var bodyBuf *bytes.Buffer
		if cfg.Log.IsDebug() {
			bodyBuf = new(bytes.Buffer)
			body = io.TeeReader(body, bodyBuf)
		}

		entities, err := esitag.Parse(body)
		if cfg.Log.IsDebug() {
			cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse",
				log.Err(err), log.Uint64("page_id", pageID), log.Int("tag_count", len(entities)),
				loghttp.Request("request", r), log.Stringer("content", bodyBuf),
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
				log.Bool("shared", shared), log.Uint64("page_id", pageID), loghttp.Request("request", r),
			)
		}
		return http.StatusInternalServerError, err
	}

	// Trigger the queries to the resource backends in parallel
	// TODO(CyS) Coalesce requests

	tags, err := (groupEntitiesResult.(esitag.Entities)).QueryResources(r)
	if err != nil {
		if cfg.Log.IsDebug() {
			cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.esiEntities.QueryResources.Error",
				log.Err(err), loghttp.Request("request", r), log.Stringer("config", cfg),
				log.Uint64("page_id", pageID),
			)
		}
		// Reported errors are mostly because of incorrect template syntax. Those gets
		// reported during first parsing.
		return http.StatusInternalServerError, err
	}

	// Calculates the correct Content-Length
	bufResW.FlushHeader(tags.DataLen())

	if _, err := bufRdr.Seek(0, 0); err != nil {
		return http.StatusInternalServerError, err
	}
	// read the 2nd time from the buffer to finally inject the content from the resource backends
	// into the HTML page
	if _, _, err := tags.InjectContent(bufRdr, w, 0); err != nil {
		return http.StatusInternalServerError, err
	}

	return code, err
}

// handleHeaderCommands allows to execute certain commands to influence the
// behaviour of the ESI tag middleware.
func handleHeaderCommands(pc *PathConfig, r *http.Request) error {
	if pc.CmdHeaderName == "" {
		return nil
	}
	switch r.Header.Get(pc.CmdHeaderName) {
	case `purge`:
		pc.purgeESICache()
	}

	return nil
}
