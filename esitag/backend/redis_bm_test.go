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

// +build esiall esiredis

package backend_test

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/garyburd/redigo/redis"
)

func BenchmarkNewRedis_Parallel(b *testing.B) {
	if isRedisRunning < 2 {
		b.Skip("Redis not running or not installed. Skipping...")
	}

	rc, err := redis.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		b.Fatal(err)
	}
	var wantValue = []byte("123,45 € Way too long")
	if rec, err := rc.Do("SET", "product_price_4711", string(wantValue)); err != nil {
		b.Fatal(err)
	} else if rec.(string) != "OK" {
		b.Fatalf("failed to set key in redis %#v", rec)
	}
	if err := rc.Close(); err != nil {
		b.Fatal(err)
	}

	runner := func(uriQueryString string) func(*testing.B) {
		return func(b *testing.B) {

			be, err := esitag.NewResourceHandler(esitag.NewResourceOptions(fmt.Sprintf("redis://127.0.0.1:6379%s", uriQueryString)))
			if err != nil {
				b.Fatalf("%+v", err)
			}
			defer be.Close()

			rfa := &esitag.ResourceArgs{
				ExternalReq: httptest.NewRequest("GET", "/", nil),
				Config: esitag.Config{
					Key:         "product_price_4711",
					Timeout:     time.Second,
					MaxBodySize: 10,
				},
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					hdr, content, err := be.DoRequest(rfa)
					if err != nil {
						b.Fatalf("%+v", err)
					}
					if hdr != nil {
						b.Fatal("header should be nil")
					}
					if have, want := len(content), 10; have != want {
						b.Errorf("Have: %q Want: %q", content, wantValue)
					}
				}
			})

			//require.NoError(t, err, "%+v", err)
			//assert.Nil(t, hdr, "Header return must be nil")
			//assert.Exactly(t, "123,45 €", string(content))
		}
	}
	b.Run("Cancellable", runner("?cancellable=1"))
	b.Run("Non__Cancel", runner("?cancellable=0"))

}
