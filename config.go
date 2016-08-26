package esi

import "time"

// Config
type Config struct {
	// Timeout global. Time when a request to a source should be canceled.
	Timeout time.Duration
	// TTL global time-to-live in the storage backend for ESI data. Defaults to
	// zero, caching disabled.
	TTL time.Duration
	// Backends Redis URLs to cache the data returned from the ESI sources.
	// Defaults to empty, caching disabled. Reads randomly from one entry and
	// writes to all entries parallel.
	Backends RedisConfigs

	// Resources used in ESI:Include to fetch data from.
	Resources map[string]*RedisConfig
}
