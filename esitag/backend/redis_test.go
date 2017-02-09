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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/esitag/backend"
	"github.com/alicebob/miniredis"
	"github.com/corestoreio/errors"
	"github.com/mitchellh/go-ps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var isRedisRunning int // 0 not init; 1 nope; 2 yes

func init() {

	pses, err := ps.Processes()
	if err != nil {
		panic(err)
	}
	isRedisRunning = 1
	for _, p := range pses {
		if "redis-server" == p.Executable() {
			isRedisRunning = 2
			break
		}
	}

	if isRedisRunning == 1 {
		if _, err := exec.LookPath("redis-server"); err != nil {
			if strings.Contains(err.Error(), exec.ErrNotFound.Error()) {
				isRedisRunning = 0 // skip tests and benchmarks
			} else {
				panic(err)
			}
		}
	}
}

func TestNewRedis(t *testing.T) {
	t.Parallel()

	if isRedisRunning < 2 {
		t.Skip("Redis not running or not installed. Skipping...")
	}

	t.Run("Failed to parse max_active", func(t *testing.T) {
		t.Parallel()
		be, err := backend.NewRedis(esitag.NewResourceOptions("redis://localHorst/?max_active=∏"))
		assert.Nil(t, be)
		assert.True(t, errors.IsNotValid(err), "%+v", err)
		assert.Error(t, err)
	})
	t.Run("Failed to parse max_idle", func(t *testing.T) {
		t.Parallel()
		be, err := backend.NewRedis(esitag.NewResourceOptions("redis://localHorst/?max_idle=∏"))
		assert.Nil(t, be)
		assert.True(t, errors.IsNotValid(err), "%+v", err)
		assert.Error(t, err)
	})
	t.Run("Failed to parse idle_timeout", func(t *testing.T) {
		t.Parallel()
		be, err := backend.NewRedis(esitag.NewResourceOptions("redis://localHorst/?idle_timeout=∏"))
		assert.Nil(t, be)
		assert.True(t, errors.IsNotValid(err), "%+v", err)
		assert.Error(t, err)
	})

	t.Run("Ping fail", func(t *testing.T) {
		t.Parallel()

		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions("redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379"))
		assert.True(t, errors.IsFatal(err), "%+v", err)
		assert.Nil(t, be)
	})

	t.Run("Ping does not fail because lazy", func(t *testing.T) {
		t.Parallel()

		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions("redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/?lazy=1"))
		if err != nil {
			t.Fatalf("There should be no error but got: %s", err)
		}
		assert.NotNil(t, be)
		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Config: esitag.Config{
				Key:         "product_price_76",
				Timeout:     time.Second,
				MaxBodySize: 10,
			},
		})
		assert.Nil(t, hdr, "header must be nil")
		assert.Nil(t, content, "content must be nil")
		assert.Contains(t, err.Error(), `dial tcp: lookup`)
	})

	t.Run("Authentication and Fetch Key OK", func(t *testing.T) {
		t.Parallel()

		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			t.Fatal(err)
		}
		defer mr.Close()
		mr.RequireAuth("MyPa55w04d")

		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions(fmt.Sprintf("redis://MrMiyagi:%s@%s", "MyPa55w04d", mr.Addr())))
		if be == nil {
			t.Fatalf("NewResourceHandler to %q returns nil %+v", mr.Addr(), err)
		}
		if err != nil {
			t.Fatalf("Redis connection %q error: %+v", mr.Addr(), err)
		}

		if err := mr.Set("product_price_4711", "123,45 € Way too long"); err != nil {
			t.Fatal(err)
		}

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Config: esitag.Config{
				Key:         "product_price_4711",
				Timeout:     time.Second,
				MaxBodySize: 10,
			},
		})
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Exactly(t, "123,45 €", string(content))
	})

	t.Run("Authentication failed", func(t *testing.T) {
		t.Parallel()

		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			t.Fatal(err)
		}
		defer mr.Close()
		mr.RequireAuth("MyPasw04d")

		be, err := backend.NewRedis(esitag.NewResourceOptions(fmt.Sprintf("redis://MrMiyagi:%s@%s", "MyPa55w04d", mr.Addr())))
		if be != nil {
			t.Fatalf("NewResourceHandler to %q returns not nil", mr.Addr())
		}
		assert.True(t, errors.IsFatal(err), "%+v", err)
	})

	// getMrBee mr == MiniRedis; Bee = Back EEnd ;-)
	getMrBee := func(t *testing.T, params ...string) (*miniredis.Miniredis, esitag.ResourceHandler, func()) {
		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			t.Fatal(err)
		}

		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions(fmt.Sprintf("redis://%s%s", mr.Addr(), strings.Join(params, "&"))))
		if be == nil {
			t.Fatalf("NewResourceHandler to %q returns nil %+v", mr.Addr(), err)
		}
		if err != nil {
			t.Fatalf("Redis connection %q error: %+v", mr.Addr(), err)
		}

		return mr, be, func() {
			mr.Close()
			if err := be.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("Cancel Request towards Redis", func(t *testing.T) {
		t.Parallel()

		mr, be, closer := getMrBee(t, "?cancellable=1")
		defer closer()

		if err := mr.Set("product_price_4711", "123,45 € Way too long"); err != nil {
			t.Fatal(err)
		}

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Config: esitag.Config{
				Key:         "product_price_4711",
				Timeout:     time.Microsecond,
				MaxBodySize: 10,
			},
		})
		require.EqualError(t, errors.Cause(err), context.DeadlineExceeded.Error(), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, string(content), "Content should be empty")
	})

	t.Run("Fetch Key NotFound (no-cancel)", func(t *testing.T) {
		t.Parallel()

		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Config: esitag.Config{
				Key:         "product_price_4711",
				Timeout:     time.Second,
				MaxBodySize: 100,
			},
		})
		require.True(t, errors.IsNotFound(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, content, "Content must be empty")
	})

	t.Run("Fetch Key NotFound (cancel)", func(t *testing.T) {
		t.Parallel()

		_, be, closer := getMrBee(t, "?cancellable=1")
		defer closer()

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Config: esitag.Config{
				Key:         "product_price_4711",
				Timeout:     time.Second,
				MaxBodySize: 100,
			},
		})
		require.True(t, errors.IsNotFound(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, content, "Content must be empty")
	})

	t.Run("Key empty error", func(t *testing.T) {
		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{})
		require.True(t, errors.IsEmpty(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("ExternalReq empty error", func(t *testing.T) {
		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			Config: esitag.Config{
				Key: "Hello",
			},
		})
		require.True(t, errors.IsEmpty(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("Timeout empty error", func(t *testing.T) {
		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			Config: esitag.Config{
				Key: "Hello",
			},
			ExternalReq: httptest.NewRequest("GET", "/", nil),
		})
		require.True(t, errors.IsEmpty(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("MaxBodySize empty error", func(t *testing.T) {
		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Config: esitag.Config{
				Key:     "Hello",
				Timeout: time.Second,
			},
		})
		require.True(t, errors.IsEmpty(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("Key Template", func(t *testing.T) {
		mr, be, closer := getMrBee(t)
		defer closer()

		const key = `product_{{ .Req.Header.Get "X-Product-ID" }}`
		const wantContent = `<b>Awesome large Gopher plush toy</b>`
		if err := mr.Set("product_GopherPlushXXL", wantContent); err != nil {
			t.Fatal(err)
		}

		tpl, err := template.New("key_tpl").Parse(key)
		require.NoError(t, err)

		hdr, content, err := be.DoRequest(&esitag.ResourceArgs{
			ExternalReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Product-ID", "GopherPlushXXL")
				return req
			}(),
			Config: esitag.Config{
				Key:         key,
				KeyTemplate: tpl,
				Timeout:     time.Second,
				MaxBodySize: 100,
			},
		})
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Exactly(t, wantContent, string(content), "Content not equal")
	})
}

func TestRedisNewResourceHandler(t *testing.T) {
	t.Parallel()

	t.Run("URL Error", func(t *testing.T) {
		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions("redis//localhost"))
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown scheme in URL: "redis//localhost". Does not contain ://`)
	})
	t.Run("Scheme Error", func(t *testing.T) {
		be, err := esitag.NewResourceHandler(esitag.NewResourceOptions("mysql://localhost"))
		assert.Nil(t, be)
		assert.True(t, errors.IsNotSupported(err), "%+v", err)
	})
}
