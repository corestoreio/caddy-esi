// Copyright 2015-present, Cyrill @ Schumacher.fm and the CoreStore contributors
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

// +build esiall esimemcache

package backend_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/corestoreio/caddy-esi/esitag"
	"github.com/corestoreio/caddy-esi/esitag/backend"
	"github.com/corestoreio/errors"
	"github.com/mitchellh/go-ps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var isMemCacheRunning int // 0 not init; 1 nope; 2 yes

func init() {

	pses, err := ps.Processes()
	if err != nil {
		panic(err)
	}
	isMemCacheRunning = 1
	for _, p := range pses {
		if "memcached" == p.Executable() {
			isMemCacheRunning = 2
			break
		}
	}

	if isMemCacheRunning == 1 {
		if _, err := exec.LookPath("memcached"); err != nil {
			if strings.Contains(err.Error(), exec.ErrNotFound.Error()) {
				isMemCacheRunning = 0 // skip tests and benchmarks
			} else {
				panic(err)
			}
		}
	}
}

func TestNewMemCache(t *testing.T) {
	t.Parallel()

	if isMemCacheRunning < 2 {
		t.Skip("Memcache not installed or not running. Skipping ...")
	}

	var valProductPrice4711 = []byte("123,45 € Way too long")

	mc := memcache.New("localhost:11211")
	if err := mc.Set(&memcache.Item{
		Key:   "product_price_4711",
		Value: valProductPrice4711,
	}); err != nil {
		t.Fatal(err)
	}

	const wantContent = `<b>Awesome large Gopher plush toy</b>`
	if err := mc.Set(&memcache.Item{
		Key:   "product_GopherPlushXXL",
		Value: []byte(wantContent),
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("Failed to parse idle_timeout", func(t *testing.T) {
		t.Parallel()
		be, err := backend.NewMemCache(esitag.NewResourceOptions("redis://localHorst/?idle_timeout=∏"))
		assert.Nil(t, be)
		assert.True(t, errors.NotValid.Match(err), "%+v", err)
		assert.Error(t, err)
	})

	t.Run("Ping fail", func(t *testing.T) {
		t.Parallel()

		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions(
			"memcache://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:11211",
		))
		assert.True(t, errors.Fatal.Match(err), "%+v", err)
		assert.Nil(t, be)
	})

	t.Run("Ping does not fail because lazy", func(t *testing.T) {
		t.Parallel()

		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions(
			"memcache://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:11211/?lazy=1",
		))
		if err != nil {
			t.Fatalf("There should be no error but got: %s", err)
		}
		assert.NotNil(t, be)
		hdr, content, err := be.DoRequest(esitag.NewResourceArgs(
			httptest.NewRequest("GET", "/", nil),
			"",
			esitag.Config{
				Key:         "product_price_76",
				Timeout:     time.Second,
				MaxBodySize: 10,
			},
		))
		assert.Nil(t, hdr, "header must be nil")
		assert.Nil(t, content, "content must be nil")
		assert.Contains(t, err.Error(), `memcache: no servers configured or available`)
	})

	t.Run("Fetch Key OK", func(t *testing.T) {
		t.Parallel()

		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions("memcache://"))
		if be == nil {
			t.Fatalf("NewResourceHandler   returns nil %+v", err)
		}
		if err != nil {
			t.Fatalf("MemCache connection   error: %+v", err)
		}

		hdr, content, err := be.DoRequest(esitag.NewResourceArgs(
			httptest.NewRequest("GET", "/", nil),
			"",
			esitag.Config{
				Key:         "product_price_4711",
				Timeout:     time.Second,
				MaxBodySize: 10,
			},
		))
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Exactly(t, "123,45 €", string(content))
	})

	// getMrBee mr == MiniMemCache; Bee = Back EEnd ;-)
	getMrBee := func(t *testing.T, params ...string) (esitag.ResourceHandler, func()) {

		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions(fmt.Sprintf("memcache://%s", strings.Join(params, "&"))))
		if be == nil {
			t.Fatalf("NewResourceHandler returns nil %+v", err)
		}
		if err != nil {
			t.Fatalf("MemCache connection   error: %+v", err)
		}

		return be, func() {
			if err := be.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("Cancel Request towards MemCache", func(t *testing.T) {
		t.Parallel()

		be, closer := getMrBee(t, "?cancellable=1")
		defer closer()

		hdr, content, err := be.DoRequest(esitag.NewResourceArgs(
			httptest.NewRequest("GET", "/", nil),
			"",
			esitag.Config{
				Key:         "product_price_4711",
				Timeout:     time.Microsecond,
				MaxBodySize: 10,
			},
		))
		time.Sleep(5 * time.Microsecond) // mandatory
		require.EqualError(t, errors.Cause(err), context.DeadlineExceeded.Error(), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, string(content), "Content should be empty")
	})

	t.Run("Fetch Key NotFound (no-cancel)", func(t *testing.T) {
		t.Parallel()

		be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(esitag.NewResourceArgs(
			httptest.NewRequest("GET", "/", nil),
			"",
			esitag.Config{
				Key:         "product_price_4712",
				Timeout:     time.Second,
				MaxBodySize: 100,
			},
		))
		require.True(t, errors.NotFound.Match(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, content, "Content must be empty")
	})

	t.Run("Fetch Key NotFound (cancel)", func(t *testing.T) {
		t.Parallel()

		be, closer := getMrBee(t, "?cancellable=1")
		defer closer()

		hdr, content, err := be.DoRequest(esitag.NewResourceArgs(
			httptest.NewRequest("GET", "/", nil),
			"",
			esitag.Config{
				Key:         "product_price_4712",
				Timeout:     time.Second,
				MaxBodySize: 100,
			},
		))
		require.True(t, errors.NotFound.Match(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, content, "Content must be empty")
	})

	t.Run("Key empty error", func(t *testing.T) {
		be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{})
		require.True(t, errors.Empty.Match(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("ExternalReq empty error", func(t *testing.T) {
		be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			Tag: esitag.Config{
				Key: "Hello",
			},
		})
		require.True(t, errors.Empty.Match(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("Timeout empty error", func(t *testing.T) {
		be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(esitag.NewResourceArgs(
			httptest.NewRequest("GET", "/", nil),
			"",
			esitag.Config{
				Key: "Hello",
			},
		))
		require.True(t, errors.Empty.Match(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("MaxBodySize empty error", func(t *testing.T) {
		be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(esitag.NewResourceArgs(
			httptest.NewRequest("GET", "/", nil),
			"",
			esitag.Config{
				Key:     "Hello",
				Timeout: time.Second,
			},
		))
		require.True(t, errors.Empty.Match(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("Key Template (non-cancle)", func(t *testing.T) {
		be, closer := getMrBee(t)
		defer closer()

		const key = `product_{HX-Product-ID}`
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Product-ID", "GopherPlushXXL")
		ra := esitag.NewResourceArgs(
			req,
			"",
			esitag.Config{
				Key:         key,
				Timeout:     time.Second,
				MaxBodySize: 100,
			},
		).ReplaceKeyURLForTesting()

		hdr, content, err := be.DoRequest(ra)
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Exactly(t, wantContent, string(content), "Content not equal")
	})

	t.Run("Key Template (cancle)", func(t *testing.T) {
		be, closer := getMrBee(t, "?cancellable=1")
		defer closer()

		const key = `product_{HX-Product-ID}`

		hdr, content, err := be.DoRequest(esitag.NewResourceArgs(
			func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Product-ID", "GopherPlushXXL")
				return req
			}(),
			"",
			esitag.Config{
				Key:         key,
				Timeout:     time.Second,
				MaxBodySize: 100,
			},
		).ReplaceKeyURLForTesting())
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Exactly(t, wantContent, string(content), "Content not equal")
	})
}

func TestMemCacheNewResourceHandler(t *testing.T) {
	t.Parallel()

	t.Run("URL Error", func(t *testing.T) {
		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions("memcache//localhost"))
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown scheme in URL: "memcache//localhost". Does not contain ://`)
	})
	t.Run("Scheme Error", func(t *testing.T) {
		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions("mysql://localhost"))
		assert.Nil(t, be)
		assert.True(t, errors.NotSupported.Match(err), "%+v", err)
	})
}
