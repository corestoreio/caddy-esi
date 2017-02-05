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

// +build esiall esigrpc

package backend_test

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/grpclog"
)

const (
	// also stored in server file
	serverListenAddr = "grpc://127.0.0.1:50049"
)

type grpcLogTestWrap struct {
	tb testing.TB
}

func (lw grpcLogTestWrap) Fatal(args ...interface{})                 { lw.tb.Fatal(args...) }
func (lw grpcLogTestWrap) Fatalf(format string, args ...interface{}) { lw.tb.Fatalf(format, args...) }
func (lw grpcLogTestWrap) Fatalln(args ...interface{})               { lw.tb.Fatal(args...) }
func (lw grpcLogTestWrap) Print(args ...interface{})                 { lw.tb.Log(args...) }
func (lw grpcLogTestWrap) Printf(format string, args ...interface{}) { lw.tb.Logf(format, args...) }
func (lw grpcLogTestWrap) Println(args ...interface{})               { lw.tb.Log(args...) }

func TestNewGRPCClient(t *testing.T) {
	t.Parallel()

	cmd := backend.StartProcess("go", "run", "grpc_server_main.go")
	go cmd.Wait()            // waits forever until killed
	defer cmd.Process.Kill() // kills the go process but not the main started server
	// when subtests which uses grpcInsecureClient run in parallel then you have
	// to comment this out because you don't know when the sub tests finishes
	// and the GRPC server gets killed before the tests finishes.
	defer backend.KillZombieProcess("grpc_server_main")

	grpclog.SetLogger(grpcLogTestWrap{tb: t})

	t.Run("Error in ParseNoSQLURL", func(t *testing.T) {
		t.Parallel()
		cl, err := backend.NewGRPCClient(&backend.ConfigItem{
			URL: "grpc://127::01:1:90000",
		})
		if err == nil {
			t.Error("Missing required error")
		}
		if !errors.IsNotValid(err) {
			t.Errorf("error should have behaviour NotValid: %+v", err)
		}
		if cl != nil {
			t.Errorf("cl should be nil, but got: %#v", cl)
		}
	})

	t.Run("Error in timeout in query string", func(t *testing.T) {
		t.Parallel()
		cl, err := backend.NewGRPCClient(&backend.ConfigItem{
			URL: serverListenAddr + "?timeout=",
		})
		if err == nil {
			t.Error("Missing required error")
		}
		// tb.Log(err)
		if !errors.IsNotValid(err) {
			t.Errorf("error should have behaviour NotValid: %+v", err)
		}
		if cl != nil {
			t.Errorf("cl should be nil, but got: %#v", cl)
		}
	})

	t.Run("Error because ca_file not found", func(t *testing.T) {
		t.Parallel()
		cl, err := backend.NewGRPCClient(&backend.ConfigItem{
			URL: serverListenAddr + "?timeout=10s&tls=1&ca_file=testdata/non_existent.pem",
		})
		if err == nil {
			t.Error("Missing required error")
		}
		if !errors.IsFatal(err) {
			t.Errorf("error should have behaviour Fatal: %+v", err)
		}
		if cl != nil {
			t.Errorf("cl should be nil, but got: %#v", cl)
		}
	})

	t.Run("Error server unreachable", func(t *testing.T) {
		t.Parallel()
		// limit timeout to 1s otherwise we'll maybe wait too long, after 1sec
		// the context gets cancelled.
		cl, err := backend.NewGRPCClient(&backend.ConfigItem{
			URL: "grpc://127.0.0.1:81049?timeout=1s",
		})
		if err == nil {
			t.Error("Missing required error")
		}
		//tb.Log(err)
		if !errors.IsFatal(err) {
			t.Errorf("error should have behaviour Fatal: %+v", err)
		}
		if cl != nil {
			t.Errorf("cl should be nil, but got: %#v", cl)
		}
	})

	grpcInsecureClient, err := backend.NewGRPCClient(&backend.ConfigItem{
		// 60s deadline to wait until server is up and running. GRPC will do
		// a reconnection. Race detector slows down the program.
		URL: serverListenAddr + "?timeout=60s",
	})
	if err != nil {
		t.Fatalf("Whooops: %+v", err)
	}

	t.Run("Connect insecure and retrieve HTML data", func(t *testing.T) {
		const key = `should be echoed back into the content response`

		const iterations = 10
		var wg sync.WaitGroup
		wg.Add(iterations)
		for i := 0; i < iterations; i++ {
			go func(wg *sync.WaitGroup) { // food for the race detector
				defer wg.Done()

				rfa := &backend.ResourceArgs{
					ExternalReq: getExternalReqWithExtendedHeaders(),
					URL:         "grpcShoppingCart1",
					Timeout:     5 * time.Second,
					MaxBodySize: 3333,
					Key:         key,
					Log:         log.BlackHole{},
				}

				hdr, content, err := grpcInsecureClient.DoRequest(rfa)
				if err != nil {
					t.Fatalf("Woops: %+v", err)
				}
				if hdr != nil {
					t.Errorf("Header should be nil because not yet supported: %#v", hdr)
				}

				assert.Contains(t, string(content), key)
				assert.Contains(t, string(content), `<p>Arg URL: grpcShoppingCart1</p>`)

			}(&wg)
		}
		wg.Wait()

	})

	t.Run("Connect insecure and retrieve error from server", func(t *testing.T) {

		rfa := &backend.ResourceArgs{
			ExternalReq: getExternalReqWithExtendedHeaders(),
			URL:         "grpcShoppingCart2",
			Timeout:     5 * time.Second,
			MaxBodySize: 3333,
			Key:         "word error in the key triggers an error on the server",
			Log:         log.BlackHole{},
		}

		hdr, content, err := grpcInsecureClient.DoRequest(rfa)
		if hdr != nil {
			t.Errorf("Header should be nil because not yet supported: %#v", hdr)
		}
		if content != nil {
			t.Errorf("Content should be nil: %q", content)
		}
		assert.Contains(t, err.Error(), `[grpc_server] Interrupted. Detected word error in "word error in the key triggers an error on the server" for URL "grpcShoppingCart2"`)
	})
}

// BenchmarkNewGRPCClient_Parallel/Insecure-4         	   10000	    173541 ns/op	   69317 B/op	      66 allocs/op
func BenchmarkNewGRPCClient_Parallel(b *testing.B) {

	// This parent benchmark function runs only once as soon as there is another
	// sub-benchmark.
	cmd := backend.StartProcess("go", "run", "grpc_server_main.go")
	go cmd.Wait()            // waits forever until killed
	defer cmd.Process.Kill() // kills the go process but not the main started server
	defer backend.KillZombieProcess("grpc_server_main")

	grpclog.SetLogger(grpcLogTestWrap{tb: b})

	// Full integration benchmark test which starts a GRPC server and uses TCP
	// to query it on the localhost.

	const lenCartExampleHTML = 21601

	b.Run("Insecure", func(b *testing.B) {

		grpcInsecureClient, err := backend.NewGRPCClient(&backend.ConfigItem{
			// 20s deadline to wait until server is up and running. GRPC will do
			// a reconnection.
			URL: serverListenAddr + "?timeout=20s",
		})
		if err != nil {
			b.Fatalf("Whooops: %+v", err)
		}

		rfa := &backend.ResourceArgs{
			ExternalReq: getExternalReqWithExtendedHeaders(),
			URL:         "http://totally-uninteresting.what",
			Key:         `cart_example.html`,
			Timeout:     time.Second,
			MaxBodySize: 22001,
			Log:         log.BlackHole{},
		}

		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			var content []byte
			var hdr http.Header
			var err error

			for pb.Next() {
				hdr, content, err = grpcInsecureClient.DoRequest(rfa)
				if err != nil {
					b.Fatalf("%+v", err)
				}
				if hdr != nil {
					b.Fatal("Header should be nil")
				}
				if len(content) != lenCartExampleHTML {
					b.Fatalf("Want %d\nHave %d", lenCartExampleHTML, len(content))
				}
			}
		})
	})
}
