package esi

import (
	"fmt"
	"strings"
	"time"
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
	Configs
	RequestESITags
}

func NewRootConfig() *RootConfig {
	return &RootConfig{
		RequestESITags: NewRequestESITags(),
	}
}

type Configs []*Config

// Config
type Config struct {
	// Base path to match
	PathScope string

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
	}
	return nil, fmt.Errorf("[caddyesi] Unknown URL: %q. No driver defined for scheme: %q", url, scheme)
}
