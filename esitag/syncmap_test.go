// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package esitag_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/SchumacherFM/caddyesi/esitag"
)

// mapCall is a quick.Generator for calls on mapInterface.
type mapCall struct {
	key   uint64
	apply func(mapInterface) (esitag.Entities, bool)
	desc  string
}

type mapResult struct {
	value esitag.Entities
	ok    bool
}

var uint64Type = reflect.TypeOf(uint64(0))

func randKey(r *rand.Rand) uint64 {
	k, ok := quick.Value(uint64Type, r)
	if !ok {
		panic(fmt.Sprintf("quick.Value(%v, _) failed", uint64Type))
	}
	return k.Uint()
}

func randValue(r *rand.Rand) esitag.Entities {
	return make(esitag.Entities, r.Intn(100))
}

func (mapCall) Generate(r *rand.Rand, size int) reflect.Value {
	k := randKey(r)

	var (
		app  func(mapInterface) (esitag.Entities, bool)
		desc string
	)
	switch rand.Intn(4) {
	case 0:
		app = func(m mapInterface) (esitag.Entities, bool) {
			return m.Load(k)
		}
		desc = fmt.Sprintf("Load(%q)", k)

	case 1:
		v := randValue(r)
		app = func(m mapInterface) (esitag.Entities, bool) {
			m.Store(k, v)
			return nil, false
		}
		desc = fmt.Sprintf("Store(%q, %q)", k, v)

	case 2:
		v := randValue(r)
		app = func(m mapInterface) (esitag.Entities, bool) {
			return m.LoadOrStore(k, v)
		}
		desc = fmt.Sprintf("LoadOrStore(%q, %q)", k, v)

	case 3:
		app = func(m mapInterface) (esitag.Entities, bool) {
			m.Delete(k)
			return nil, false
		}
		desc = fmt.Sprintf("Delete(%q)", k)
	}

	return reflect.ValueOf(mapCall{k, app, desc})
}

func applyCalls(m mapInterface, calls []mapCall) (results []mapResult, final map[uint64]esitag.Entities) {
	for _, c := range calls {
		v, ok := c.apply(m)
		results = append(results, mapResult{v, ok})
	}

	final = make(map[uint64]esitag.Entities)
	m.Range(func(k uint64, v esitag.Entities) bool {
		final[k] = v
		return true
	})

	return results, final
}

func applyMap(calls []mapCall) ([]mapResult, map[uint64]esitag.Entities) {
	return applyCalls(new(esitag.EntitiesMap), calls)
}

func applyRWMutexMap(calls []mapCall) ([]mapResult, map[uint64]esitag.Entities) {
	return applyCalls(new(RWMutexMap), calls)
}

func TestMapMatchesRWMutex(t *testing.T) {
	if err := quick.CheckEqual(applyMap, applyRWMutexMap, nil); err != nil {
		t.Error(err)
	}
}

func TestMapMatchesDeepCopy(t *testing.T) {
	if err := quick.CheckEqual(applyMap, applyRWMutexMap, nil); err != nil {
		t.Error(err)
	}
}
