// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
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
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/corestoreio/caddy-esi/bufpool"
	"github.com/corestoreio/caddy-esi/esitag"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	loghttp "github.com/corestoreio/log/http"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"golang.org/x/sync/singleflight"
)

const avgESITagsPerPage = 5 // just a guess

// Middleware implements the Tag tag middleware
type Middleware struct {
	Group singleflight.Group
	// Root the Server root
	Root string
	//FileSys  jails the requests to site root with a mock file system
	FileSys http.FileSystem
	// Next HTTP handler in the chain
	Next httpserver.Handler

	// PathConfigs The list of Tag configurations for each path prefix and theirs
	// caches.
	PathConfigs
	// coalesce guarantees the execution of one backend request when n-external
	// incoming requests occur. Pointer type not needed.
	coalesce singleflight.Group
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
				log.Bool("is_response_allowed", false), loghttp.Request("request", r), log.Stringer("config", cfg),
			)
		}
		return mw.Next.ServeHTTP(w, r) // go on ...
	}
	if err := handleHeaderCommands(cfg, w, r); err != nil {
		// clears the Tag tags
		return http.StatusInternalServerError, err
	}

	pageID, entities := cfg.ESITagsByRequest(r)
	if entities == nil || len(entities) == 0 {
		// Slow path because Tag cache tag is empty and we need to analyse the
		// buffer.
		return mw.serveBuffered(cfg, pageID, w, r)
	}

	////////////////////////////////////////////////////////////////////////////////
	// Proceed from map, filled with the parsed Tag tags.

	var logR *http.Request
	if cfg.Log.IsInfo() || cfg.Log.IsDebug() { // avoids race condition when logging
		// TODO(CyS) logging this request can be avoided because we only need to
		// trace a request ID and log somewhere which request ID belongs to
		// which printed request for debugging
		logR = loghttp.ShallowCloneRequest(r)
	}

	chanTag := make(chan esitag.DataTag)
	go func() {
		var wg *sync.WaitGroup
		if entities.HasCoalesce() {
			wg = new(sync.WaitGroup)
			var coaEnt esitag.Entities
			coaEnt, entities = entities.SplitCoalesce()
			// variable entities will be reused after go func() to query the
			// non-coalesce resources.

			var logR2 *http.Request
			if cfg.Log.IsInfo() || cfg.Log.IsDebug() { // avoids race condition when logging
				logR2 = loghttp.ShallowCloneRequest(logR)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				coaID := coaEnt.UniqueID()
				doRes, _, _ := mw.coalesce.Do(strconv.FormatUint(coaID, 10), func() (interface{}, error) {
					coaChanTag := make(chan esitag.DataTag)
					// wow this is ugly (3 level of goroutines) but for now the
					// best I can come up with. but not using coalesce will
					// consume less memory than with the code in the previous
					// version of QueryResources.
					go func() {
						if err := coaEnt.QueryResources(coaChanTag, r); err != nil {
							if cfg.Log.IsInfo() {
								cfg.Log.Info("caddyesi.Middleware.ServeHTTP.coaEnt.QueryResources.Error",
									log.Err(err), log.Uint64("page_id", pageID), log.Uint64("entities_coalesce_id", coaID),
									loghttp.Request("request", logR2),
								)
							}
						}
						if cfg.Log.IsDebug() {
							cfg.Log.Info("caddyesi.Middleware.ServeHTTP.coaEnt.QueryResources.Once",
								log.Uint64("page_id", pageID), log.Uint64("entities_coalesce_id", coaID),
								log.Stringer("coalesce_entities", coaEnt), log.Stringer("non_coalesce_entities", entities),
								loghttp.Request("request", logR2),
							)
						}
						close(coaChanTag)
					}()
					tags := esitag.NewDataTagsCapped(avgESITagsPerPage)
					for tag := range coaChanTag {
						tags.Slice = append(tags.Slice, tag)
					}
					return tags, nil
				})
				for _, tag := range doRes.(*esitag.DataTags).Slice {
					chanTag <- tag
				}
			}()
		}

		// trigger the DoRequests and query all backend resources in
		// parallel. Errors are mostly of cancelled client requests which
		// the context propagates.
		err := entities.QueryResources(chanTag, r)
		if err != nil {
			if cfg.Log.IsInfo() {
				cfg.Log.Info("caddyesi.Middleware.ServeHTTP.entities.QueryResources.Error",
					log.Err(err), loghttp.Request("request", logR), log.Stringer("config", cfg),
					log.Uint64("page_id", pageID),
				)
			}
		}
		if wg != nil {
			wg.Wait()
		}
		close(chanTag)
	}()
	return mw.Next.ServeHTTP(responseWrapInjector(chanTag, w), r)
}

// serveBuffered creates a http.ResponseWriter buffer, calls the next handler,
// waits until the buffer has been filled, parses the buffer for Tag tags,
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
		bufResW.TriggerRealWrite(0)
		if _, err := bufResW.Write(buf.Bytes()); err != nil {
			return http.StatusInternalServerError, err
		}
		return code, nil
	}

	// Parse the buffer to find Tag tags. First buffer Read happens within this
	// Group.Do block. We make sure with the Group.Do call that Tag tags for a
	// specific page ID gets only parsed once, even if multiple requests are
	// coming in to for same page. Therefore you should make sure that your
	// pageID has been calculated correctly.

	// run a performance load test to see if it's worth to switch to Group.DoChan
	groupEntitiesResult, err, shared := mw.Group.Do(strconv.FormatUint(pageID, 10), func() (interface{}, error) {

		entities, err := esitag.Parse(newSimpleReader(buf.Bytes()))
		if cfg.Log.IsDebug() {
			const contentMaxLength = 512
			var content string
			if buf.Len() < contentMaxLength {
				content = buf.String()
			} else {
				content = buf.String()[:contentMaxLength]
			}

			cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.ESITagsByRequest.Parse",
				log.Err(err), log.Uint64("page_id", pageID), log.Int("tag_count", len(entities)),
				loghttp.Request("request", r), log.String("content_512", content),
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
				log.Err(err), log.String("scope", cfg.Scope),
				log.Bool("shared", shared), log.Uint64("page_id", pageID), loghttp.Request("request", r),
			)
		}
		return http.StatusInternalServerError, err
	}

	// Trigger the queries to the resource backends in parallel
	// TODO(CyS) Coalesce requests

	cTags := make(chan esitag.DataTag, 1)
	go func() {
		if err := (groupEntitiesResult.(esitag.Entities)).QueryResources(cTags, r); err != nil {
			if cfg.Log.IsDebug() {
				cfg.Log.Debug("caddyesi.Middleware.ServeHTTP.esiEntities.QueryResources.Error",
					log.Err(err), loghttp.Request("request", r), log.Stringer("config", cfg),
					log.Uint64("page_id", pageID),
				)
			}
			// todo: might leak senitive data now because the error gets not handled
			// Reported errors are mostly because of incorrect template syntax. Those gets
			// reported during first parsing.
			//return http.StatusInternalServerError, err
		}
		close(cTags)
	}()

	tags := esitag.NewDataTagsCapped(avgESITagsPerPage)
	for t := range cTags {
		tags.Slice = append(tags.Slice, t)
	}

	// Calculates the correct Content-Length and enables now the real writing to the
	// client.
	bufResW.TriggerRealWrite(tags.DataLen())

	// restore original order as occurred in the HTML document.
	sort.Sort(tags)

	// read the 2nd time from the buffer to finally inject the content from the resource backends
	// into the HTML page
	if _, err := tags.InjectContent(buf.Bytes(), bufResW); err != nil {
		return http.StatusInternalServerError, err
	}

	return code, err
}

// handleHeaderCommands allows to execute certain commands to influence the
// behaviour of the Tag tag middleware.
func handleHeaderCommands(pc *PathConfig, w http.ResponseWriter, r *http.Request) (err error) {
	if pc.CmdHeaderName == "" {
		return nil
	}
	var logLevel string

	switch r.Header.Get(pc.CmdHeaderName) {
	case `purge`:
		prevItemsInMap := pc.purgeESICache()
		w.Header().Set(pc.CmdHeaderName, fmt.Sprintf("purge-ok-%d", prevItemsInMap))
	case `log-debug`:
		logLevel = "debug"
	case `log-info`:
		logLevel = "info"
	case `log-none`:
		logLevel = "none"
	}

	if logLevel != "" {
		// TODO: check for race conditions
		pc.esiMU.Lock()
		prevLevel := pc.LogLevel
		pc.LogLevel = logLevel
		err = setupLogger(pc)
		pc.esiMU.Unlock()
		if err != nil {
			return errors.Wrap(err, "[caddyesi] handleHeaderCommands.setupLogger")
		}
		w.Header().Set(pc.CmdHeaderName, fmt.Sprintf("log-%s-ok", prevLevel))
	}

	return nil
}
