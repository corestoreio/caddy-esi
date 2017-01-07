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
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esikv"
	"github.com/alicebob/miniredis"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// https://github.com/alicebob/miniredis

func TestNewRedis(t *testing.T) {
	t.Parallel()

	t.Run("Ping fail", func(t *testing.T) {
		be, err := esikv.NewResourceHandler("redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0")
		assert.True(t, errors.IsFatal(err), "%+v", err)
		assert.Nil(t, be)
	})

	t.Run("Fetch Key OK", func(t *testing.T) {

		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			t.Fatal(err)
		}
		defer mr.Close()

		if err := mr.Set("product_price_4711", "123,45 € Way too long"); err != nil {
			t.Fatal(err)
		}

		be, err := esikv.NewResourceHandler(fmt.Sprintf("redis://%s", mr.Addr()))
		assert.NoError(t, err, "%+v", err)
		defer be.Close()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Key:         "product_price_4711",
			MaxBodySize: 10,
		})
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Exactly(t, "123,45 €", string(content))
	})

	t.Run("Fetch Key OK but value not set, should return all nil", func(t *testing.T) {

		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			t.Fatal(err)
		}
		defer mr.Close()

		be, err := esikv.NewResourceHandler(fmt.Sprintf("redis://%s", mr.Addr()))
		assert.NoError(t, err, "%+v", err)
		defer be.Close()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{
			ExternalReq: httptest.NewRequest("GET", "/", nil),
			Key:         "product_price_4711",
		})
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("Key empty error", func(t *testing.T) {

		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			t.Fatal(err)
		}
		defer mr.Close()

		be, err := esikv.NewResourceHandler(fmt.Sprintf("redis://%s", mr.Addr()))
		assert.NoError(t, err, "%+v", err)
		defer be.Close()

		hdr, content, err := be.DoRequest(&backend.ResourceArgs{})
		require.True(t, errors.IsEmpty(err), "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Nil(t, content, "Content must be nil")
	})

	t.Run("Key Template", func(t *testing.T) {

		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			t.Fatal(err)
		}
		defer mr.Close()

		const key = `product_{{ .Req.Header.Get "X-Product-ID" }}`
		const wantContent = `<b>Awesome large Gopher plush toy</b>`
		if err := mr.Set("product_GopherPlushXXL", wantContent); err != nil {
			t.Fatal(err)
		}

		be, err := esikv.NewResourceHandler(fmt.Sprintf("redis://%s", mr.Addr()))
		assert.NoError(t, err, "%+v", err)
		defer be.Close()

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
		})
		require.NoError(t, err, "%+v", err)
		assert.Nil(t, hdr, "Header return must be nil")
		assert.Exactly(t, wantContent, string(content), "Content not equal")
	})
}

func TestNewResourceHandler(t *testing.T) {
	t.Parallel()

	t.Run("URL Error", func(t *testing.T) {
		be, err := esikv.NewResourceHandler("redis//localhost")
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown URL: "redis//localhost". Does not contain ://`)
	})
	t.Run("Scheme Error", func(t *testing.T) {
		be, err := esikv.NewResourceHandler("mysql://localhost")
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown URL: "mysql://localhost". No driver defined for scheme: "mysql"`)
	})
}

func TestParseRedisURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw          string
		wantAddress  string
		wantPassword string
		wantDB       int
		wantErr      bool
	}{
		{
			"localhost",
			"",
			"",
			0,
			true, // "invalid redis URL scheme",
		},
		// The error message for invalid hosts is diffferent in different
		// versions of Go, so just check that there is an error message.
		{
			"redis://weird url",
			"",
			"",
			0,
			true,
		},
		{
			"redis://foo:bar:baz",
			"",
			"",
			0,
			true,
		},
		{
			"http://www.google.com",
			"",
			"",
			0,
			true, // "invalid redis URL scheme: http",
		},
		{
			"redis://localhost:6379/abc123",
			"",
			"",
			0,
			true, // "invalid database: abc123",
		},
		{
			"redis://localhost:6379/123",
			"localhost:6379",
			"",
			123,
			false,
		},
		{
			"redis://:6379/123",
			"localhost:6379",
			"",
			123,
			false,
		},
		{
			"redis://",
			"localhost:6379",
			"",
			0,
			false,
		},
		{
			"redis://192.168.0.234/123",
			"192.168.0.234:6379",
			"",
			123,
			false,
		},
		{
			"redis://192.168.0.234/",
			"",
			"",
			0,
			true,
		},
		{
			"redis://empty:SuperSecurePa55w0rd@192.168.0.234/3",
			"192.168.0.234:6379",
			"SuperSecurePa55w0rd",
			3,
			false,
		},
	}
	for i, test := range tests {

		haveAddress, havePW, haveDB, haveErr := esikv.ParseRedisURL(test.raw)

		if have, want := haveAddress, test.wantAddress; have != want {
			t.Errorf("(%d) Address: Have: %v Want: %v", i, have, want)
		}
		if have, want := havePW, test.wantPassword; have != want {
			t.Errorf("(%d) Password: Have: %v Want: %v", i, have, want)
		}
		if have, want := haveDB, test.wantDB; have != want {
			t.Errorf("(%d) DB: Have: %v Want: %v", i, have, want)
		}
		if test.wantErr {
			if have, want := test.wantErr, haveErr != nil; have != want {
				t.Errorf("(%d) Error: Have: %v Want: %v", i, have, want)
			}
		} else {
			if haveErr != nil {
				t.Errorf("(%d) Did not expect an Error: %+v", i, haveErr)
			}
		}
	}
}
