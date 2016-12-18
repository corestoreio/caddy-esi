package caddyesi

import (
	"net/http"
	"sync"
	"time"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/pierrec/xxHash/xxHash64"
)

const DefaultTimeOut = 30 * time.Second

// PathConfigs contains the configuration for each path prefix
type PathConfigs []*PathConfig

// ConfigForPath selects in the ServeHTTP function the config for a path.
func (pc PathConfigs) ConfigForPath(r *http.Request) *PathConfig {
	for _, c := range pc {
		if httpserver.Path(r.URL.Path).Matches(c.Scope) { // not negated
			// match also all sub paths ... ?
			return c
		}
	}
	return nil
}

// PathConfig per path prefix
type PathConfig struct {
	// Base path to match used as path prefix
	Scope string
	// Timeout global. Time when a request to a source should be canceled.
	// Default value from the constant DefaultTimeOut.
	Timeout time.Duration
	// TTL global time-to-live in the storage backend for ESI data. Defaults to
	// zero, caching globally disabled until an ESI tag contains the TTL
	// attribute.
	TTL time.Duration
	// RequestIDSource defines a slice of possible parameters which gets
	// extracted from the http.Request object. All these parameters will be used
	// to extract the values and calculate a unique hash for the current request
	// to identify the request in the cache.
	RequestIDSource []string
	// AllowedMethods list of all allowed methods, defaults to GET
	AllowedMethods []string

	// Caches stores content from a e.g. micro service but only when the TTL has
	// been set within an ESI tag. Caches gets set during configuration parsing.
	Caches

	// KVFetchers the map key is the alias name in the CaddyFile for a Key-Value
	// service. The value is the already instantiated object but with a lazy
	// connection initialization. This map gets created during configuration
	// parsing and the default value is nil.
	KVServices map[string]KVFetcher

	muRes sync.RWMutex
	// Resources used in ESI:Include to fetch data from a e.g. micro service.
	// string is the src attribute in an ESI tag to identify a resource.
	// These entries gets set during parsing a HTML page.
	Resources map[string]ResourceFetcher

	muESI sync.RWMutex
	// esiCache identifies all parsed ESI tags in a page for specific path
	// prefix. uint64 represents the hash for the current request calculated byt
	// requestID function,
	esiCache map[uint64]esitag.Entities
}

// NewPathConfig creates a configuration for a unique path prefix and
// initializes the internal maps.
func NewPathConfig() *PathConfig {
	return &PathConfig{
		Timeout:   DefaultTimeOut,
		Resources: make(map[string]ResourceFetcher),
		esiCache:  make(map[uint64]esitag.Entities),
	}
}

// ESITagsByRequest selects in the ServeHTTP function all ESITags identified byt
// its requestID.
func (pc *PathConfig) ESITagsByRequest(r *http.Request) (requestID uint64, t esitag.Entities) {
	requestID = pc.requestID(r)
	pc.muESI.RLock()
	t = pc.esiCache[requestID]
	pc.muESI.RUnlock()
	return
}

// StoreESITags as an ESI tag slice with its associated request ID to the
// internal ESI cache and maybe overwrites an existing entry.
func (pc *PathConfig) StoreESITags(requestID uint64, t esitag.Entities) {
	pc.muESI.Lock()
	defer pc.muESI.Unlock()
	pc.esiCache[requestID] = t
}

// IsRequestAllowed decides if a request should be processed.
func (pc *PathConfig) IsRequestAllowed(r *http.Request) bool {
	for _, m := range pc.AllowedMethods {
		if r.Method == m {
			return true
		}
	}
	return r.Method == http.MethodGet
}

var defaultRequestIDSource = [...]string{"host", "path"}

// requestID uses configs to extract certain parameters from the request
func (pc *PathConfig) requestID(r *http.Request) uint64 {
	src := pc.RequestIDSource
	if len(src) == 0 {
		src = defaultRequestIDSource[:]
	}

	h, ok := requestID(src, r)
	if !ok {
		h, _ = requestID(defaultRequestIDSource[:], r)
	}
	return h
}

func requestID(source []string, r *http.Request) (_ uint64, ok bool) {
	const (
		requestIDHeader = `header`
		requestIDCookie = `cookie`
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
			case requestIDCookie:
				if keks, _ := r.Cookie(keySuffix); keks != nil {
					_, _ = buf.WriteString(keks.Value)
				}
			case requestIDHeader:
				if v := r.Header.Get(keySuffix); v != "" {
					_, _ = buf.WriteString(v)
				}
			}
		}

		switch key {
		case "remoteaddr":
			_, _ = buf.WriteString(r.RemoteAddr)
		case "realip":
			_, _ = buf.Write(helpers.RealIP(r))
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
