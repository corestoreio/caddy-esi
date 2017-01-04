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

package esicache

import (
	"context"
	"github.com/corestoreio/errors"
	"sync"
	"time"
)

// Cacher used to cache the response of a micro service as found in the src
// attribute of an ESI tag. But the Cacher gets only involved if the additional
// attribute ttl has been set for each ESI tag. A Cacher must be thread safe.
type Cacher interface {
	Set(key string, value []byte, expiration time.Duration) error
	Get(key string) ([]byte, error)
}

func NewCacher(url string) (Cacher, error) {
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

var MainRegistry = &registry{
	caches: make(Caches, 0, 2),
}

type registry struct {
	mu sync.RWMutex
	// kvServices the map key is the alias name in the CaddyFile for a Key-Value
	// service. The value is the already instantiated object but with a lazy
	// connection initialization. This map gets created during configuration
	// parsing and the default value is nil.
	caches Caches
}

func (r *registry) Get(ctx context.Context, alias string, key []byte) error {

	return nil
}

// Register registers a new key-value service
func (r *registry) Register(url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, err := NewCacher(url)
	if err != nil {
		return errors.Wrapf(err, "[esikv] NewCacher URL %q", url)
	}
	r.caches = append(r.caches, c)

	return nil
}

// Aliases returns a sorted list of the available aliases for the loaded
// services.
func (r *registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.caches)
}

// Clear removes all cache service objects
func (r *registry) Clear() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.caches = make(Caches, 0, 2)
}
