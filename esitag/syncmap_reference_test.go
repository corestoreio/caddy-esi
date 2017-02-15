// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package esitag_test

import (
	"sync"

	"github.com/SchumacherFM/caddyesi/esitag"
)

// This file contains reference map implementations for unit-tests.

// mapInterface is the interface EntitiesMap implements.
type mapInterface interface {
	Load(uint64) (esitag.Entities, bool)
	Store(key uint64, value esitag.Entities)
	LoadOrStore(key uint64, value esitag.Entities) (actual esitag.Entities, loaded bool)
	Delete(uint64)
	Range(func(key uint64, value esitag.Entities) (shouldContinue bool))
}

// RWMutexMap is an implementation of mapInterface using a sync.RWMutex.
type RWMutexMap struct {
	mu    sync.RWMutex
	dirty map[uint64]esitag.Entities
}

func (m *RWMutexMap) Load(key uint64) (value esitag.Entities, ok bool) {
	m.mu.RLock()
	value, ok = m.dirty[key]
	m.mu.RUnlock()
	return
}

func (m *RWMutexMap) Store(key uint64, value esitag.Entities) {
	m.mu.Lock()
	if m.dirty == nil {
		m.dirty = make(map[uint64]esitag.Entities)
	}
	m.dirty[key] = value
	m.mu.Unlock()
}

func (m *RWMutexMap) LoadOrStore(key uint64, value esitag.Entities) (actual esitag.Entities, loaded bool) {
	m.mu.Lock()
	actual, loaded = m.dirty[key]
	if !loaded {
		actual = value
		if m.dirty == nil {
			m.dirty = make(map[uint64]esitag.Entities)
		}
		m.dirty[key] = value
	}
	m.mu.Unlock()
	return actual, loaded
}

func (m *RWMutexMap) Delete(key uint64) {
	m.mu.Lock()
	delete(m.dirty, key)
	m.mu.Unlock()
}

func (m *RWMutexMap) Range(f func(key uint64, value esitag.Entities) (shouldContinue bool)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.dirty {
		if !f(k, v) {
			break
		}
	}
}
