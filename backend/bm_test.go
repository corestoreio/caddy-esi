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

package backend_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esitesting"
)

var benchmarkResourceArgs_PrepareForwardHeaders []string

func BenchmarkResourceArgs_PrepareForwardHeaders(b *testing.B) {

	rfa := &backend.ResourceArgs{
		ExternalReq:       getExternalReqWithExtendedHeaders(),
		ForwardHeadersAll: true,
	}

	b.Run("All", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkResourceArgs_PrepareForwardHeaders = rfa.PrepareForwardHeaders()
		}
		if have, want := len(benchmarkResourceArgs_PrepareForwardHeaders), 20; have != want {
			b.Fatalf("Have: %v Want: %v", have, want)
		}
	})

	b.Run("Two", func(b *testing.B) {
		rfa.ForwardHeadersAll = false
		rfa.ForwardHeaders = []string{"Cookie", "user-agent"}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkResourceArgs_PrepareForwardHeaders = rfa.PrepareForwardHeaders()
		}
		if have, want := len(benchmarkResourceArgs_PrepareForwardHeaders), 4; have != want {
			b.Fatalf("Have: %v Want: %v", have, want)
		}
	})
}

var benchmarkResourceArgs_PrepareReturnHeaders http.Header

func BenchmarkResourceArgs_PrepareReturnHeaders(b *testing.B) {

	rfa := &backend.ResourceArgs{
		ExternalReq:      getExternalReqWithExtendedHeaders(),
		ReturnHeadersAll: true,
	}

	b.Run("All", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkResourceArgs_PrepareReturnHeaders = rfa.PrepareReturnHeaders(resourceRespWithExtendedHeaders)
		}
		if have, want := len(benchmarkResourceArgs_PrepareReturnHeaders), 6; have != want {
			b.Fatalf("Have: %v Want: %v", have, want)
		}
	})

	b.Run("Two", func(b *testing.B) {
		rfa.ReturnHeadersAll = false
		rfa.ReturnHeaders = []string{"Set-Cookie", "x-sdch-encode"}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchmarkResourceArgs_PrepareReturnHeaders = rfa.PrepareReturnHeaders(resourceRespWithExtendedHeaders)
		}
		if have, want := len(benchmarkResourceArgs_PrepareReturnHeaders), 2; have != want {
			b.Fatalf("Have: %v Want: %v", have, want)
		}
	})
}

func BenchmarkResourceArgs_TemplateToURL(b *testing.B) {
	const key = `product_{{ .Req.Header.Get "X-Product-ID" }}`
	const wantKey = `product_GopherPlushXXL`
	tpl, err := template.New("key_tpl").Parse(key)
	if err != nil {
		b.Fatalf("%+v", err)
	}

	rfa := &backend.ResourceArgs{
		ExternalReq: func() *http.Request {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Product-ID", "GopherPlushXXL")
			return req
		}(),
		Key:         key,
		KeyTemplate: tpl,
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		have, err := rfa.TemplateToURL(tpl)
		if err != nil {
			b.Fatalf("%+v", err)
		}
		if have != wantKey {
			b.Errorf("Have: %v Want: %v", have, wantKey)
		}

	}
}

// BenchmarkResourceArgs_MarshalEasyJSON-4   	  300000	      4844 ns/op	    1922 B/op	       6 allocs/op
func BenchmarkResourceArgs_MarshalEasyJSON(b *testing.B) {

	rfa := &backend.ResourceArgs{
		URL:            "https://corestore.io",
		ExternalReq:    getExternalReqWithExtendedHeaders(),
		Timeout:        5 * time.Second,
		MaxBodySize:    50000,
		Key:            "a_r€dis_ky",
		TTL:            33 * time.Second,
		ForwardHeaders: []string{"X-Cart-Id", "Cookie"},
		ReturnHeaders:  []string{"Set-Cookie"},
	}
	var haveData []byte
	for i := 0; i < b.N; i++ {
		var err error
		haveData, err = rfa.MarshalJSON()
		if err != nil {
			b.Fatal(err)
		}
	}
	if len(haveData) != 1176 {
		b.Fatalf("Incorret JSON: Incorrect length %d", len(haveData))
	}
}

// Full in memory bench test without touching the HDD
// BenchmarkNewFetchHTTP_Parallel-4   	   50000	     29915 ns/op	   65713 B/op	      39 allocs/op
func BenchmarkNewFetchHTTP_Parallel(b *testing.B) {

	// file size ~22KB
	wantContent, err := ioutil.ReadFile("testdata/cart_example.html")
	if err != nil {
		b.Fatal(err)
	}

	fh := backend.NewFetchHTTP(esitesting.NewHTTPTripBytes(200, wantContent, nil))

	rfa := &backend.ResourceArgs{
		ExternalReq: getExternalReqWithExtendedHeaders(),
		URL:         "http://totally-uninteresting.what",
		Timeout:     time.Second,
		MaxBodySize: 22001,
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var content []byte
		var hdr http.Header
		var err error

		for pb.Next() {
			hdr, content, err = fh.DoRequest(rfa)
			if err != nil {
				b.Fatalf("%+v", err)
			}
			if hdr != nil {
				b.Fatal("Header should be nil")
			}
			if len(content) != len(wantContent) {
				b.Fatalf("Want %q\nHave %q", wantContent, content)
			}
		}
	})
}

// Benchmark reads from the HDD.
// BenchmarkNewFetchShellExec_Parallel-4   	    1000	   2418130 ns/op	   32713 B/op	     130 allocs/op <- no goroutine
// BenchmarkNewFetchShellExec_Parallel-4   	    1000	   2409384 ns/op	   33137 B/op	     138 allocs/op <- with goroutine
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2591573 ns/op	   34581 B/op	     140 allocs/op
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2563336 ns/op	   33895 B/op	     137 allocs/op
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2702138 ns/op	   76056 B/op	     147 allocs/op no pool
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2567728 ns/op	   33883 B/op	     137 allocs/op pool
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2473684 ns/op	   31218 B/op	     105 allocs/op full path to binary
// BenchmarkNewFetchShellExec_Parallel-4   	     500	   2561815 ns/op	   32589 B/op	     108 allocs/op full path + goroutine

func BenchmarkNewFetchShellExec_Parallel(b *testing.B) {
	wantContent, err := ioutil.ReadFile("testdata/cart_example.html")
	if err != nil {
		b.Fatal(err)
	}

	fh := backend.NewFetchShellExec()

	// ProTip: providing the full path to the script/binary reduces lookup time
	// and searching in the env PATH variable. So we use /bin/cat
	rfa := &backend.ResourceArgs{
		ExternalReq: getExternalReqWithExtendedHeaders(),
		URL:         "sh:///bin/cat testdata/cart_example.html",
		Timeout:     time.Second,
		MaxBodySize: 22001,
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var content []byte
		var hdr http.Header
		var err error

		for pb.Next() {
			hdr, content, err = fh.DoRequest(rfa)
			if err != nil {
				b.Fatalf("%+v", err)
			}
			if hdr != nil {
				b.Fatal("Header should be nil")
			}
			if len(content) != len(wantContent) {
				b.Fatalf("Want %q\nHave %q", wantContent, content)
			}
		}
	})
}
