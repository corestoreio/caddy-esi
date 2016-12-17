package caddyesi

import (
	"net/http"
	"sync"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag"
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
	// esiCache identifies all parsed ESI tags in a page for specific path prefix.
	// uint64 represents the hash for the current request.
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
func (pc *PathConfig) ESITagsByRequest(r *http.Request) (t esitag.Entities) {
	pc.muESI.RLock()
	t = pc.esiCache[pc.requestID(r)]
	pc.muESI.RUnlock()
	return
}

func (pc *PathConfig) isRequestAllowed(r *http.Request) bool {
	for _, m := range pc.AllowedMethods {
		if r.Method == m {
			return true
		}
	}
	return r.Method == http.MethodGet
}

// requestID uses configs to extract certain parameters from the request
func (pc *PathConfig) requestID(r *http.Request) uint64 {
	// for now this should be enough, we can optimize it later or add more stuff, like headers

	// pc.RequestIDSource

	l := len(r.URL.Host) + len(r.URL.Path)
	buf := make([]byte, l)
	n := copy(buf, r.URL.Host)
	n += copy(buf[n:], r.URL.Path)
	buf = buf[:n]
	return xxHash64.Checksum(buf, uint64(l))
}
