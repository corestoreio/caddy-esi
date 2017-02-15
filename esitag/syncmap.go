// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package esitag

import (
	"sync"
	"sync/atomic"
)

// EntitiesMap is a concurrent map with amortized-constant-time operations.
// It is safe for multiple goroutines to call a EntitiesMap's methods concurrently.
//
// The zero EntitiesMap is valid and empty.
//
// A EntitiesMap must not be copied after first use.
type EntitiesMap struct {
	mu sync.Mutex

	// clean is a copy of the map's contents that will never be overwritten, and
	// is thus safe for concurrent lookups without additional synchronization.
	//
	// A nil clean map indicates that the current map contents are stored in the
	// dirty field instead.
	// If clean is non-nil, its contents are up-to-date.
	//
	// clean is always safe to load, but must only be stored with mu held.
	clean atomic.Value // map[uint64]Entities

	// dirty is a copy of the map to which all writes occur.
	//
	// A nil dirty map indicates that the current map contents are either empty or
	// stored in the clean field.
	//
	// If the dirty map is nil, the next write to the map will initialize it by
	// making a deep copy of the clean map, then setting the clean map to nil.
	dirty map[uint64]Entities

	// misses counts the number of Load calls for which the clean map was nil
	// since the last write.
	//
	// Once enough Load misses have occurred to cover the cost of a copy, the
	// dirty map will be promoted to clean and any subsequent writes will make
	// a new copy.
	misses int
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (m *EntitiesMap) Load(key uint64) (value Entities, ok bool) {
	clean, _ := m.clean.Load().(map[uint64]Entities)
	if clean != nil {
		value, ok = clean[key]
		return value, ok
	}

	m.mu.Lock()
	if m.dirty == nil {
		clean, _ := m.clean.Load().(map[uint64]Entities)
		if clean == nil {
			// Completely empty — promote to clean immediately.
			m.clean.Store(map[uint64]Entities{})
		} else {
			value, ok = clean[key]
		}
		m.mu.Unlock()
		return value, ok
	}
	value, ok = m.dirty[key]
	m.missLocked()
	m.mu.Unlock()
	return value, ok
}

// Store sets the value for a key.
func (m *EntitiesMap) Store(key uint64, value Entities) {
	m.mu.Lock()
	m.dirtyLocked()
	m.dirty[key] = value
	m.mu.Unlock()
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *EntitiesMap) LoadOrStore(key uint64, value Entities) (actual Entities, loaded bool) {
	// Avoid locking if it's a clean hit.
	clean, _ := m.clean.Load().(map[uint64]Entities)
	actual, loaded = clean[key]
	if loaded {
		return actual, true
	}

	m.mu.Lock()
	if m.dirty == nil {
		// Reload clean in case it changed while we were waiting on m.mu.
		clean, _ := m.clean.Load().(map[uint64]Entities)
		actual, loaded = clean[key]
		if loaded {
			m.mu.Unlock()
			return actual, true
		}
	} else {
		actual, loaded = m.dirty[key]
		if loaded {
			m.missLocked()
			m.mu.Unlock()
			return actual, true
		}
	}

	m.dirtyLocked()
	m.dirty[key] = value
	actual = value
	m.mu.Unlock()
	return actual, false
}

// Delete deletes the value for a key.
func (m *EntitiesMap) Delete(key uint64) {
	m.mu.Lock()
	m.dirtyLocked()
	delete(m.dirty, key)
	m.mu.Unlock()
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Calls to other EntitiesMap methods may block until Range returns.
// The function f must not call any other methods on the EntitiesMap.
//
// Range does not necessarily correspond to any consistent snapshot of the EntitiesMap's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently, Range may reflect any mapping for that key
// from any point during the Range call.
func (m *EntitiesMap) Range(f func(key uint64, value Entities) bool) {
	clean, _ := m.clean.Load().(map[uint64]Entities)
	if clean == nil {
		m.mu.Lock()
		if m.dirty == nil {
			clean, _ = m.clean.Load().(map[uint64]Entities)
			if clean == nil {
				// Completely empty — add an empty map to bypass m.mu next time.
				m.clean.Store(map[uint64]Entities{})
			}
		} else {
			// Range is already O(N), so a call to Range amortizes an entire copy of
			// the map.  If it is dirty, we can promote it to clean immediately!
			clean = m.dirty
			m.clean.Store(clean)
			m.dirty = nil
		}
		m.mu.Unlock()
	}

	for k, v := range clean {
		if !f(k, v) {
			break
		}
	}
}

func (m *EntitiesMap) missLocked() {
	if m.misses++; m.misses >= len(m.dirty) {
		m.clean.Store(m.dirty)
		m.dirty = nil
	}
}

// dirtyLocked prepares the map for a subsequent write.
// It ensures that the dirty field is non-nil and clean is nil by making a deep
// copy of clean.
func (m *EntitiesMap) dirtyLocked() {
	m.misses = 0
	if m.dirty != nil {
		return
	}

	clean, _ := m.clean.Load().(map[uint64]Entities)
	m.dirty = make(map[uint64]Entities, len(clean))
	for k, v := range clean {
		m.dirty[k] = v
	}
	m.clean.Store(map[uint64]Entities(nil))
}
