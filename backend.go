package caddyesi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SchumacherFM/caddyesi/esiredis"
)

// Cacher used to cache the response of a micro service as found in the src
// attribute of an ESI tag. But the Cacher gets only involved if the additional
// attribute ttl has been set for each ESI tag. A Cacher must be thread safe.
type Cacher interface {
	Set(key string, value []byte, expiration time.Duration) error
	Get(key string) ([]byte, error)
}

func newCacher(url string) (Cacher, error) {
	// same logic as newKVFetcher
	return nil, nil
}

// Caches gets set during config reading and implements Cacher interface
type Caches []Cacher

func (c Caches) Set(key string, value []byte, expiration time.Duration) error {
	// write to all
	return nil
}

func (c Caches) Get(key string) ([]byte, error) {
	// race condition which cache returns first
	return nil, nil
}

// ResourceFetcher fetches content from a micro service
type ResourceFetcher interface {
	Get(*http.Request) ([]byte, error)
	Close() error
}

// KVFetcher represents a KeyValue fetching service which can query a remote
// service, like Redis, for a key and returns a byte slice which gets injected
// later into the returned HTML.
type KVFetcher interface {
	// Get can use the context to cancel the request
	Get(ctx context.Context, key []byte) ([]byte, error)
	Close() error // Closes the connection during server restart
}

func newKVFetcher(url string) (KVFetcher, error) {
	idx := strings.Index(url, "://")
	if idx < 0 {
		return nil, fmt.Errorf("[caddyesi] Unknown URL: %q. Does not contain ://", url)
	}
	scheme := url[:idx]

	switch scheme {
	case "redis":
		r, err := esiredis.New(url)
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
