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

// +build esiall

// above build tag triggers inclusion of all backend resource connectors

package caddyesi

import (
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/alicebob/miniredis"
	"github.com/corestoreio/errors"
)

func TestPluginSetup_Backends(t *testing.T) {
	t.Parallel()

	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	t.Run("With timeout, ttl and 1x Cacher", testPluginSetup(
		`esi {
			timeout 5ms
			ttl 10ms
			cache redis://`+mr.Addr()+`/0
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		1,   // cache length
		nil, // kv services []string
		nil,
	))

	t.Run("With timeout, ttl and 2x Cacher", testPluginSetup(
		`esi {
			timeout 5ms
			ttl 10ms
			cache redis://`+mr.Addr()+`/0
			cache redis://`+mr.Addr()+`/1
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		2,   // cache length
		nil, // kv services []string
		nil,
	))

	esiCfg, clean := esitesting.WriteXMLTempFile(t, ResourceItems{
		NewResourceItem(`redis://`+mr.Addr()+`/0`, "backend"),
	})
	defer clean()
	t.Run("With timeout, ttl and resources", testPluginSetup(
		`esi {
			timeout 5ms
			ttl 10ms
			resources `+esiCfg+`
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		0,                   // cache length
		[]string{"backend"}, // kv services []string
		nil,
	))

	esiCfg, clean = esitesting.WriteXMLTempFile(t, ResourceItems{
		NewResourceItem(`redis//`+mr.Addr()+`/0`, "backend"), // missing colon
	})
	defer clean()

	t.Run("Invalid KVService URL", testPluginSetup(
		`esi {
			timeout 5ms
			ttl 10ms
			resources `+esiCfg+`
		}`,
		nil,
		0,   // cache length
		nil, // kv services []string
		errors.IsNotValid,
	))

	esiCfg, clean = esitesting.WriteXMLTempFile(t, ResourceItems{
		NewResourceItem(`redis://`+mr.Addr()+`/0`, "backend"),
		NewResourceItem(`redis://`+mr.Addr()+`/1`, "backend"),
	})
	defer clean()
	t.Run("Overwrite duplicate key in KVServices", testPluginSetup(
		`esi {
			timeout 5ms
			ttl 10ms
			resources `+esiCfg+`
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		0,                   // cache length
		[]string{"backend"}, // kv services []string
		nil,
	))

	esiCfg, clean = esitesting.WriteXMLTempFile(t, ResourceItems{
		NewResourceItem(`redis://`+mr.Addr()+`/0`, "redis1"),
		NewResourceItem(`redis://`+mr.Addr()+`/1`, "redis2"),
	})
	defer clean()
	t.Run("Create two KVServices", testPluginSetup(
		`esi {
			timeout 5ms
			ttl 10ms
			resources `+esiCfg+`
		}`,
		PathConfigs{
			&PathConfig{
				Scope:   "/",
				Timeout: time.Millisecond * 5,
				TTL:     time.Millisecond * 10,
			},
		},
		0, // cache length
		[]string{"redis1", "redis2"}, // kv services []string
		nil,
	))

	esiCfg1, clean := esitesting.WriteXMLTempFile(t, ResourceItems{
		NewResourceItem(`redis://`+mr.Addr()+`/2`, "redisAWS1"),
		NewResourceItem(`redis://`+mr.Addr()+`/3`, "redisLocal1"),
		NewResourceItem(`redis://`+mr.Addr()+`/1`, "redisLocal2"),
	})
	defer clean()
	esiCfg2, clean := esitesting.WriteXMLTempFile(t, ResourceItems{
		NewResourceItem(`redis://`+mr.Addr()+`/3`, "redisLocal3"),
	})
	defer clean()
	t.Run("2x esi directives with different KVServices", testPluginSetup(
		`esi /catalog/product {
		   timeout 122ms
		   ttl 123ms
		   log_file stderr
		   log_level debug
		   resources `+esiCfg1+`
		}
		esi /checkout/cart {
		   timeout 131ms
		   ttl 132ms
			resources `+esiCfg2+`
		}`,
		PathConfigs{
			&PathConfig{
				Scope:    "/catalog/product",
				Timeout:  time.Millisecond * 122,
				TTL:      time.Millisecond * 123,
				LogFile:  "stderr",
				LogLevel: "debug",
			},
			&PathConfig{
				Scope:   "/checkout/cart",
				Timeout: time.Millisecond * 131,
				TTL:     time.Millisecond * 132,
			},
		},
		0, // cache length
		[]string{"redisAWS1", "redisLocal1", "redisLocal2", "redisLocal3"}, // kv services []string
		nil,
	))

}
