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
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/corestoreio/caddy-esi/bufpool"
	"github.com/corestoreio/caddy-esi/esitag"
	"github.com/corestoreio/caddy-esi/helper"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/pierrec/xxHash/xxHash64"
)

// DefaultTimeOut to a backend resource
const DefaultTimeOut = 20 * time.Second

// DefaultMaxBodySize the body size of a retrieved request to a resource. 5 MB
// is a lot of text.
const DefaultMaxBodySize uint64 = 5 << 20

// DefaultOnError default error message when a backend service cannot be
// requested. Only when config value on_error in Caddyfile has not been
// supplied.
const DefaultOnError = `Resource not available`

// PathConfigs contains the configuration for each path prefix
type PathConfigs []*PathConfig

// ConfigForPath selects in the ServeHTTP function the config for a path.
func (pc PathConfigs) ConfigForPath(r *http.Request) *PathConfig {
	for _, c := range pc {
		if httpserver.Path(r.URL.Path).Matches(c.Scope) { // not negated
			// match also all sub paths ...
			return c
		}
	}
	return nil
}

// String prints debug information. Very slow ...
func (pc PathConfigs) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "PathConfig Count: %d", len(pc))
	buf.WriteRune('\n')
	for _, c := range pc {
		buf.WriteString(c.String())
		buf.WriteRune('\n')
	}
	return buf.String()
}

// PathConfig per path prefix
type PathConfig struct {
	// Scope sets the base path to match used as path prefix
	Scope string

	// MaxBodySize defaults to 5MB and limits the size of the returned body from a
	// backend resource.
	MaxBodySize uint64
	// Timeout global. Time when a request to a source should be canceled.
	// Default value from the constant DefaultTimeOut.
	Timeout time.Duration
	// TTL global time-to-live in the storage backend for Tag data. Defaults to
	// zero, caching globally disabled until an Tag tag or this configuration
	// value contains the TTL attribute.
	TTL time.Duration
	// CmdHeaderName if set allows to execute certain maintenance functions to
	// e.g. purge the cache. For security reasons an empty string means, feature
	// has been disabled.
	CmdHeaderName string

	// PageIDSource defines a slice of possible parameters which gets extracted
	// from the http.Request object. All these parameters will be used to
	// extract the values and calculate a unique hash for the current requested
	// page to identify the already parsed Tag tags in the cache.
	PageIDSource []string
	// AllowedMethods list of all benchIsResponseAllowed methods, defaults to GET
	AllowedMethods []string
	// OnError gets output when a request to a backend service fails.
	OnError []byte
	// LogFile where to write the log output? Either any file name or stderr or
	// stdout. If empty logging disabled.
	LogFile string

	esiMU sync.RWMutex
	// LogLevel can have the values info, debug, fatal. If empty logging disabled.
	LogLevel string
	// Log gets set up during setup
	Log log.Logger
	// esiCache identifies all parsed Tag tags in a page for specific path
	// prefix. uint64 represents the hash for the current request calculated by
	// pageID function. Long term "bug": Maybe we need here another algorithm
	// instead of the map. Due to a higher granularity of the pageID the map
	// gets filled fast without dropping old entries. This will blow up the
	// memory.
	esiCache map[uint64]esitag.Entities // TODO after refacotring other stuff replace with EntitiesMap but run before benchmarks and after ;-)
}

// NewPathConfig creates a configuration for a unique path prefix and
// initializes the internal maps.
func NewPathConfig() *PathConfig {
	return &PathConfig{
		Timeout:  DefaultTimeOut,
		esiCache: make(map[uint64]esitag.Entities),
	}
}

func (pc *PathConfig) parseOnError(val string) (err error) {
	var fileExt string
	if li := strings.LastIndexByte(val, '.'); li > 0 {
		fileExt = strings.ToLower(val[li+1:])
	}

	switch fileExt {
	case "html", "htm", "xml", "txt", "json":
		pc.OnError, err = ioutil.ReadFile(filepath.Clean(val))
		if err != nil {
			return errors.NewFatalf("[caddyesi] PathConfig.parseOnError. Failed to process %q with error: %s. Scope %q", val, err, pc.Scope)
		}
	default:
		pc.OnError = []byte(val)
	}

	return nil
}

// ESITagsByRequest selects in the ServeHTTP function all ESITags identified by
// their pageIDs. Returns a nil t when the entry does not exists.
func (pc *PathConfig) ESITagsByRequest(r *http.Request) (pageID uint64, t esitag.Entities) {
	pageID = pc.pageID(r)
	pc.esiMU.RLock()
	t = pc.esiCache[pageID]
	pc.esiMU.RUnlock()
	return
}

// UpsertESITags processes each Tag entity to update their default values with
// the supplied global PathConfig value. Then inserts the Tag entities with its
// associated page ID in the internal Tag cache. These writes to esitag.Entity
// happens in a locked environment. So there should be no race condition.
func (pc *PathConfig) UpsertESITags(pageID uint64, entities esitag.Entities) {

	for _, et := range entities {

		et.Log = pc.Log

		if len(et.OnError) == 0 {
			et.OnError = pc.OnError
		}
		// add here the KVFetcher ...

		// create sync.pool of arguments for the resources. Now with all correct
		// default values.
		et.SetDefaultConfig(esitag.Config{
			Log:         pc.Log,
			MaxBodySize: pc.MaxBodySize,
			Timeout:     pc.Timeout,
			TTL:         pc.TTL,
		})
	}

	pc.esiMU.Lock()
	pc.esiCache[pageID] = entities
	pc.esiMU.Unlock()
}

// IsRequestAllowed decides if a request should be processed based on the
// request method. The benchIsResponseAllowed response content-type is text only.
func (pc *PathConfig) IsRequestAllowed(r *http.Request) bool {

	if len(pc.AllowedMethods) == 0 {
		return r.Method == http.MethodGet
	}
	for _, m := range pc.AllowedMethods {
		if r.Method == m {
			return true
		}
	}
	return false
}

var defaultPageIDSource = [...]string{"host", "path"}

// pageID uses the configuration to extract certain parameters from the request
// to generate a hash to identify a page.
func (pc *PathConfig) pageID(r *http.Request) uint64 {
	src := pc.PageIDSource
	if len(src) == 0 {
		src = defaultPageIDSource[:]
	}

	h, ok := pageID(src, r)
	if !ok {
		h, _ = pageID(defaultPageIDSource[:], r)
	}
	return h
}

func pageID(source []string, r *http.Request) (_ uint64, ok bool) {
	const (
		pageIDConfigHeader = `header`
		pageIDConfigCookie = `cookie`
	)

	buf := bufpool.Get()
	defer bufpool.Put(buf)

	for _, key := range source {
		{
			var keyPrefix string
			var keySuffix string
			if len(key) > 7 {
				// "Header" and "Cookie" are equally long which makes things easier
				// Cookie-__Host-user_session_same_site
				// Header-Server
				keyPrefix = key[:6] // Contains e.g. "header" or "cookie"
				keySuffix = key[7:] // Contains e.g. "__Host-user_session_same_site" or "Server"
			}

			switch keyPrefix {
			case pageIDConfigCookie:
				if keks, _ := r.Cookie(keySuffix); keks != nil {
					_, _ = buf.WriteString(keks.Value)
				}
			case pageIDConfigHeader:
				if v := r.Header.Get(keySuffix); v != "" {
					_, _ = buf.WriteString(v)
				}
			}
		}

		switch key {
		case "remoteaddr":
			_, _ = buf.WriteString(r.RemoteAddr)
		case "realip":
			_, _ = buf.WriteString(helper.RealIP(r))
		case "scheme":
			_, _ = buf.WriteString(r.URL.Scheme)
		case "host":
			_, _ = buf.WriteString(r.URL.Host)
		case "path":
			_, _ = buf.WriteString(r.URL.Path)
		case "rawpath":
			_, _ = buf.WriteString(r.URL.RawPath)
		case "rawquery":
			_, _ = buf.WriteString(r.URL.RawQuery)
		case "url":
			_, _ = buf.WriteString(r.URL.String())

		}
	}

	l := uint64(buf.Len())
	if l == 0 {
		return 0, false
	}
	return xxHash64.Checksum(buf.Bytes(), l), true
}

// String used for log information output
func (pc *PathConfig) String() string {
	pc.esiMU.RLock()
	el := len(pc.esiCache)
	pc.esiMU.RUnlock()
	return fmt.Sprintf("Scope:%q; MaxBodySize:%d; Timeout:%s; PageIDSource:%v; AllowedMethods:%v; LogFile:%q; LogLevel:%q; EntityCount: %d",
		pc.Scope, pc.MaxBodySize, pc.Timeout, pc.PageIDSource, pc.AllowedMethods, pc.LogFile, pc.LogLevel, el,
	)
}

func (pc *PathConfig) purgeESICache() (itemsInMap int) {
	pc.esiMU.Lock()
	itemsInMap = len(pc.esiCache)
	pc.esiCache = make(map[uint64]esitag.Entities)
	pc.esiMU.Unlock()
	if pc.Log.IsDebug() {
		pc.Log.Debug("caddyesi.PathConfig.purgeESICache", log.String("path_scope", pc.Scope))
	}
	return
}

// isResponseAllowed uses https://golang.org/pkg/net/http/#DetectContentType
// it must read at least 512 bytes.
func isResponseAllowed(buf []byte) bool {
	fileType := http.DetectContentType(buf)
	return strings.HasPrefix(fileType, "text/")
}
