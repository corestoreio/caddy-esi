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

package esikv_test

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esikv"
	"github.com/alicebob/miniredis"
)

// 50000	     34904 ns/op	     941 B/op	      32 allocs/op <-- with deadline ctx
// 50000	     30739 ns/op	     492 B/op	      23 allocs/op <-- without deadline
func BenchmarkNewRedis_Parallel(b *testing.B) {
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		b.Fatal(err)
	}
	defer mr.Close()

	var wantValue = []byte("123,45 € Way too long")
	if err := mr.Set("product_price_4711", string(wantValue)); err != nil {
		b.Fatal(err)
	}

	be, err := esikv.NewResourceHandler(fmt.Sprintf("redis://%s", mr.Addr()))
	if err != nil {
		b.Fatalf("%+v", err)
	}
	defer be.Close()

	rfa := &backend.ResourceArgs{
		ExternalReq: httptest.NewRequest("GET", "/", nil),
		Key:         "product_price_4711",
		Timeout:     time.Second,
		MaxBodySize: 10,
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
