package esi

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mholt/caddy/caddyhttp/httpserver"
	"github.com/pierrec/xxHash/xxHash64"
)

type Backender interface {
	Set(key, val []byte) error
	Get(key []byte) ([]byte, error)
}

type Backends []Backender

// Resourcer fetches a key from a resource and returns its value.
type Resourcer interface {
	Get(key []byte) ([]byte, error)
}

type RootConfig struct {
	PathConfigs
	mu    sync.RWMutex
	cache map[uint64]ESITags
}

func NewRootConfig() *RootConfig {
	return &RootConfig{
		cache: make(map[uint64]ESITags),
	}
}

// ESITagsByRequest selects in the ServeHTTP function all ESITags identified byt
// its requestID.
func (rc *RootConfig) ESITagsByRequest(r *http.Request) (t ESITags) {
	rc.mu.RLock()
	t = rc.cache[requestID(r)]
	rc.mu.RUnlock()
	return
}

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

// Config
type PathConfig struct {
	// Base path to match
	Scope string

	// Timeout global. Time when a request to a source should be canceled.
	Timeout time.Duration
	// TTL global time-to-live in the storage backend for ESI data. Defaults to
	// zero, caching disabled.
	TTL time.Duration
	// Backends Redis URLs to cache the data returned from the ESI sources.
	// Defaults to empty, caching disabled. Reads randomly from one entry and
	// writes to all entries parallel.
	Backends

	// Resources used in ESI:Include to fetch data from.
	// string is the src attribute in an ESI tag
	Resources map[string]Resourcer
}

func requestID(r *http.Request) uint64 {
	// for now this should be enough, we can optimize it later or add more stuff, like headers
	l := len(r.URL.Host) + len(r.URL.Path)
	buf := make([]byte, l)
	n := copy(buf, r.URL.Host)
	n += copy(buf[n:], r.URL.Path)
	buf = buf[:n]
	return xxHash64.Checksum(buf, uint64(l))
}

func parseBackendUrl(url string) (Backender, error) {
	idx := strings.Index(url, "://")
	if idx < 0 {
		return nil, fmt.Errorf("[caddyesi] Unknown URL: %q. Does not contain ://", url)
	}
	scheme := url[:idx]

	switch scheme {
	case "redis":
		r, err := NewRedis(url)
		if err != nil {
			return nil, fmt.Errorf("[caddyesi] Failed to parse Backend Redis URL: %q with Error %s", url, err)
		}
		return r, nil
		//case "memcache":
		//case "mysql":
		//case "pgsql":
		//case "grpc":
	}
	return nil, fmt.Errorf("[caddyesi] Unknown URL: %q. No driver defined for scheme: %q", url, scheme)
}
