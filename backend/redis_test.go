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

// +build all redis

package backend_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/alicebob/miniredis"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedis(t *testing.T) {
	t.Parallel()

	t.Run("Failed to parse max_active", func(t *testing.T) {
		t.Parallel()
		be, err := backend.NewRedis(backend.NewConfigItem("redis://localHorst/?max_active=∏"))
		assert.Nil(t, be)
		assert.Error(t, err)
	})
	t.Run("Failed to parse max_idle", func(t *testing.T) {
		t.Parallel()
		be, err := backend.NewRedis(backend.NewConfigItem("redis://localHorst/?max_idle=∏"))
		assert.Nil(t, be)
		assert.Error(t, err)
	})
	t.Run("Failed to parse idle_timeout", func(t *testing.T) {
		t.Parallel()
		be, err := backend.NewRedis(backend.NewConfigItem("redis://localHorst/?idle_timeout=∏"))
		assert.Nil(t, be)
		assert.Error(t, err)
	})

	t.Run("Ping fail", func(t *testing.T) {
		t.Parallel()

		be, err := backend.NewResourceHandler(backend.NewConfigItem("redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379"))
		assert.True(t, errors.IsFatal(err), "%+v", err)
		assert.Nil(t, be)
	})

	t.Run("Ping does not fail because lazy", func(t *testing.T) {
		t.Parallel()

		be, err := backend.NewResourceHandler(backend.NewConfigItem("redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/?lazy=1"))
		if err != nil {
			t.Fatalf("There should be no error but got: %s", err)
		}
		assert.NotNil(t, be)
		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Key:         "product_price_76",
			Timeout:     time.Second,
			MaxBodySize: 10,
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

		be, err := backend.NewResourceHandler(backend.NewConfigItem(fmt.Sprintf("redis://MrMiyagi:%s@%s", "MyPa55w04d", mr.Addr())))
		if be == nil {
			t.Fatalf("NewResourceHandler to %q returns nil %+v", mr.Addr(), err)
		}
		if err != nil {
			t.Fatalf("Redis connection %q error: %+v", mr.Addr(), err)
		}

		if err := mr.Set("product_price_4711", "123,45 € Way too long"); err != nil {
			t.Fatal(err)
		}

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Key:         "product_price_4711",
			Timeout:     time.Second,
			MaxBodySize: 10,
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

		be, err := backend.NewRedis(backend.NewConfigItem(fmt.Sprintf("redis://MrMiyagi:%s@%s", "MyPa55w04d", mr.Addr())))
		if be != nil {
			t.Fatalf("NewResourceHandler to %q returns not nil", mr.Addr())
		}
		assert.True(t, errors.IsFatal(err), "%+v", err)
	})

	// getMrBee mr == MiniRedis; Bee = Back EEnd ;-)
	getMrBee := func(t *testing.T, params ...string) (*miniredis.Miniredis, backend.ResourceHandler, func()) {
		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			t.Fatal(err)
		}

		be, err := backend.NewResourceHandler(backend.NewConfigItem(fmt.Sprintf("redis://%s%s", mr.Addr(), strings.Join(params, "&"))))
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

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Key:         "product_price_4711",
			Timeout:     time.Microsecond,
			MaxBodySize: 10,
		})
		require.EqualError(t, errors.Cause(err), context.DeadlineExceeded.Error(), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, string(content), "Content should be empty")
	})

	t.Run("Fetch Key NotFound (no-cancel)", func(t *testing.T) {
		t.Parallel()

		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Key:         "product_price_4711",
			Timeout:     time.Second,
			MaxBodySize: 100,
		})
		require.True(t, errors.IsNotFound(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, content, "Content must be empty")
	})

	t.Run("Fetch Key NotFound (cancel)", func(t *testing.T) {
		t.Parallel()

		_, be, closer := getMrBee(t, "?cancellable=1")
		defer closer()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Key:         "product_price_4711",
			Timeout:     time.Second,
			MaxBodySize: 100,
		})
		require.True(t, errors.IsNotFound(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Empty(t, content, "Content must be empty")
	})

	t.Run("Key empty error", func(t *testing.T) {
		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{})
		require.True(t, errors.IsEmpty(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("ExternalReq empty error", func(t *testing.T) {
		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			Key: "Hello",
		})
		require.True(t, errors.IsEmpty(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("Timeout empty error", func(t *testing.T) {
		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			Key:         "Hello",
			ExternalReq: httptest.NewRequest("GET", "/", nil),
		})
		require.True(t, errors.IsEmpty(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("MaxBodySize empty error", func(t *testing.T) {
		_, be, closer := getMrBee(t)
		defer closer()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			Key:         "Hello",
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Timeout:     time.Second,
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

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			ExternalReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.Header.Set("X-Product-ID", "GopherPlushXXL")
				return req
			}(),
			Key:         key,
			KeyTemplate: tpl,
			Timeout:     time.Second,
			MaxBodySize: 100,
		})
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Exactly(t, wantContent, string(content), "Content not equal")
	})
}

func TestNewResourceHandler(t *testing.T) {
	t.Parallel()

	t.Run("URL Error", func(t *testing.T) {
		be, err := backend.NewResourceHandler(backend.NewConfigItem("redis//localhost"))
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown URL: "redis//localhost". Does not contain ://`)
	})
	t.Run("Scheme Error", func(t *testing.T) {
		be, err := backend.NewResourceHandler(backend.NewConfigItem("mysql://localhost"))
		assert.Nil(t, be)
		assert.True(t, errors.IsNotSupported(err), "%+v", err)
	})
}

func TestParseRedisURL(t *testing.T) {
	t.Parallel()

	var defaultPoolConnectionParameters = map[string][]string{
		"db":           {"0"},
		"max_active":   {"10"},
		"max_idle":     {"400"},
		"idle_timeout": {"240s"},
		"cancellable":  {"0"},
	}

	runner := func(raw string, wantAddress string, wantPassword string, wantParams url.Values, wantErr bool) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			haveAddress, havePW, params, haveErr := backend.ParseRedisURL(raw)
			if wantErr {
				if have, want := wantErr, haveErr != nil; have != want {
					t.Errorf("(%q)\nError: Have: %v Want: %v\n%+v", t.Name(), have, want, haveErr)
				}
				return
			}

			if haveErr != nil {
				t.Errorf("(%q) Did not expect an Error: %+v", t.Name(), haveErr)
			}

			if have, want := haveAddress, wantAddress; have != want {
				t.Errorf("(%q) Address: Have: %v Want: %v", t.Name(), have, want)
			}
			if have, want := havePW, wantPassword; have != want {
				t.Errorf("(%q) Password: Have: %v Want: %v", t.Name(), have, want)
			}
			if wantParams == nil {
				wantParams = defaultPoolConnectionParameters
			}

			for k := range wantParams {
				assert.Exactly(t, wantParams.Get(k), params.Get(k), "Test %q Parameter %q", t.Name(), k)
			}
		}
	}
	t.Run("invalid redis URL scheme none", runner("localhost", "", "", nil, true))
	t.Run("invalid redis URL scheme http", runner("http://www.google.com", "", "", nil, true))
	t.Run("invalid redis URL string", runner("redis://weird url", "", "", nil, true))
	t.Run("too many colons in URL", runner("redis://foo:bar:baz", "", "", nil, true))
	t.Run("ignore path in URL", runner("redis://localhost:6379/abc123", "localhost:6379", "", nil, false))
	t.Run("URL contains only scheme", runner("redis://", "localhost:6379", "", nil, false))

	t.Run("set DB with hostname", runner(
		"redis://localh0Rst:6379/?db=123",
		"localh0Rst:6379",
		"",
		map[string][]string{
			"db":           {"123"},
			"max_active":   {"10"},
			"max_idle":     {"400"},
			"idle_timeout": {"240s"},
			"cancellable":  {"0"},
		},
		false))
	t.Run("set DB without hostname", runner(
		"redis://:6379/?db=345",
		"localhost:6379",
		"",
		map[string][]string{
			"db":           {"345"},
			"max_active":   {"10"},
			"max_idle":     {"400"},
			"idle_timeout": {"240s"},
			"cancellable":  {"0"},
		},
		false))
	t.Run("URL contains IP address", runner(
		"redis://192.168.0.234/?db=123",
		"192.168.0.234:6379",
		"",
		map[string][]string{
			"db":           {"123"},
			"max_active":   {"10"},
			"max_idle":     {"400"},
			"idle_timeout": {"240s"},
			"cancellable":  {"0"},
		},
		false))
	t.Run("URL contains password", runner(
		"redis://empty:SuperSecurePa55w0rd@192.168.0.234/?db=3",
		"192.168.0.234:6379",
		"SuperSecurePa55w0rd",
		map[string][]string{
			"db":           {"3"},
			"max_active":   {"10"},
			"max_idle":     {"400"},
			"idle_timeout": {"240s"},
			"cancellable":  {"0"},
		},
		false))
	t.Run("Apply all params", runner(
		"redis://empty:SuperSecurePa55w0rd@192.168.0.234/?db=4&max_active=2718&max_idle=3141&idle_timeout=5h3s&cancellable=1",
		"192.168.0.234:6379",
		"SuperSecurePa55w0rd",
		map[string][]string{
			"db":           {"4"},
			"max_active":   {"2718"},
			"max_idle":     {"3141"},
			"idle_timeout": {"5h3s"},
			"cancellable":  {"1"},
		},
		false))
}
