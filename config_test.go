// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
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

package caddyesi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/corestoreio/caddy-esi/helper"
	"github.com/stretchr/testify/assert"
)

var _ fmt.Stringer = (*PathConfigs)(nil)
var _ fmt.Stringer = (*PathConfig)(nil)

func TestPathConfigs_String(t *testing.T) {
	pc := PathConfigs{
		&PathConfig{
			Scope:       "/catalog/product",
			MaxBodySize: 4,
		},
		&PathConfig{
			Scope:       "/checkout/cart",
			MaxBodySize: 3,
		},
	}
	assert.Exactly(t,
		"PathConfig Count: 2\nScope:\"/catalog/product\"; MaxBodySize:4; Timeout:0s; PageIDSource:[]; AllowedMethods:[]; LogFile:\"\"; LogLevel:\"\"; EntityCount: 0\nScope:\"/checkout/cart\"; MaxBodySize:3; Timeout:0s; PageIDSource:[]; AllowedMethods:[]; LogFile:\"\"; LogLevel:\"\"; EntityCount: 0\n",
		pc.String())
}

const weirdLongURL = `https://app.usunu.com/-/login?u=https%3A%2F%2Fapp.usunu.com%2F0%2Fsearch%2F2385944396396%2F81453167684176&e=emailaddress%40gmail.com&passive=1`

func TestPathConfig_PageID(t *testing.T) {
	t.Parallel()

	runner := func(pageIDSource []string, r *http.Request, wantSum uint64) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			pc := NewPathConfig()
			pc.PageIDSource = pageIDSource

			if have, want := pc.pageID(r), wantSum; have != want {
				t.Errorf("Test %q\nHave: %x Want: %x\nHave: %d Want: %d", t.Name(), have, want, have, want)
			}
		}
	}

	t.Run("Default Host & Path (empty)", runner(
		nil,
		httptest.NewRequest("GET", "/", nil),
		0x7a6e1f1822179273,
	))
	t.Run("Default Host & Path (test)", runner(
		nil,
		httptest.NewRequest("GET", "/test", nil),
		0x2e3a61d5bfffd7d2,
	))
	t.Run("Default Host & Path (tEst)", runner(
		nil,
		httptest.NewRequest("GET", "/tEst", nil),
		0x9a11d866604d519f,
	))
	t.Run("Cookie correct", runner(
		[]string{"cookie-xtestKeks"},
		func() *http.Request {
			r := httptest.NewRequest("GET", "/test", nil)
			r.AddCookie(&http.Cookie{Name: "xtestKeks", Value: "xVal"})
			return r
		}(),
		0xf12f535bf90d7060,
	))
	t.Run("Cookie config wrong, fall back to default", runner(
		[]string{"Cookie-xtestKeks"},
		func() *http.Request {
			r := httptest.NewRequest("GET", "/test", nil)
			r.AddCookie(&http.Cookie{Name: "xtestKeks", Value: "xVal"})
			return r
		}(),
		0x2e3a61d5bfffd7d2, // equal to default because Cookie is upper case
	))
	t.Run("Header correct", runner(
		[]string{"header-xtestHeader"},
		func() *http.Request {
			r := httptest.NewRequest("GET", "/test", nil)
			r.Header.Set("xtestHeader", "xVal2")
			return r
		}(),
		0xbcf6bbff89b2d7d6,
	))
	t.Run("Header config wrong, fall back to default", runner(
		[]string{"Header-xtestHeader"},
		func() *http.Request {
			r := httptest.NewRequest("GET", "/test", nil)
			r.Header.Set("xtestHeader", "xVal2")
			return r
		}(),
		0x2e3a61d5bfffd7d2, // equal to default because Header is upper case
	))
	t.Run("remote addr", runner(
		[]string{"remoteaddr"},
		func() *http.Request {
			r := httptest.NewRequest("GET", "/test", nil)
			r.RemoteAddr = "127.0.0.2"
			return r
		}(),
		0xf024aa02b95193e,
	))
	t.Run("realip", runner(
		[]string{"realip"},
		func() *http.Request {
			r := httptest.NewRequest("GET", "/test", nil)
			r.Header.Set(helper.XClusterClientIP, "127.0.0.2")
			return r
		}(),
		0x11155771612facfc, // hash of the byte slice of cluster client IP
	))
	t.Run("scheme", runner(
		[]string{"scheme"},
		httptest.NewRequest("GET", "https://caddyserver.com/test", nil),
		0x909acbb899ed37e6,
	))

	t.Run("host", runner(
		[]string{"host"},
		httptest.NewRequest("GET", weirdLongURL, nil),
		0x16a46f35c998d63d,
	))
	t.Run("path", runner(
		[]string{"path"},
		httptest.NewRequest("GET", weirdLongURL, nil),
		0x163de6d2f60202bc, // path is: /-/login
	))
	t.Run("rawpath", runner(
		[]string{"rawpath"},
		httptest.NewRequest("GET", weirdLongURL, nil),
		0xb4b967239b2b0817, // rawpath is: app.usunu.com/-/login
	))
	t.Run("rawquery", runner(
		[]string{"rawquery"},
		httptest.NewRequest("GET", weirdLongURL, nil),
		0xb08e9c9fd24079b4, // rawquery is: u=https%3A%2F%2Fapp.usunu.com%2F0%2Fsearch%2F2385944396396%2F81453167684176&e=emailaddress%40gmail.com&passive=1
	))
	t.Run("url", runner(
		[]string{"url"},
		httptest.NewRequest("GET", weirdLongURL, nil),
		0x6c7360d1c2978e84, // full url
	))

	t.Run("default page01.html", runner(
		nil,
		httptest.NewRequest("GET", "http://127.0.0.1:2017/page01.html", nil),
		0xe7fcc1b160b213c2,
	))
	t.Run("default page02.html", runner(
		nil,
		httptest.NewRequest("GET", "http://127.0.0.1:2017/page02.html", nil),
		0xb5a0ad5009522352,
	))

}

var benchmarkPageID uint64

func BenchmarkPageID(b *testing.B) {
	r := httptest.NewRequest("GET", "/catalog/product/id/42342342/price/134.231/stock/1/camera.html", nil)
	pc := NewPathConfig()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkPageID = pc.pageID(r)
	}
}

func BenchmarkPageID_FullURL(b *testing.B) {
	r := httptest.NewRequest("GET", weirdLongURL, nil)

	pc := NewPathConfig()
	pc.PageIDSource = []string{"url"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkPageID = pc.pageID(r)
	}
}

func BenchmarkPageID_Cookie(b *testing.B) {
	r := httptest.NewRequest("GET", weirdLongURL, nil)
	r.AddCookie(&http.Cookie{Name: "xtestKeks", Value: "xVal"})

	pc := NewPathConfig()
	pc.PageIDSource = []string{"cookie-xtestKeks"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkPageID = pc.pageID(r)
	}
}

func TestPathConfig_isRequestAllowed(t *testing.T) {
	t.Parallel()
	runner := func(allowedMethods []string, r *http.Request, want bool) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			pc := NewPathConfig()
			pc.AllowedMethods = allowedMethods
			assert.Exactly(t, want, pc.IsRequestAllowed(r))
		}
	}
	t.Run("Default GET benchIsResponseAllowed", runner(
		nil,
		httptest.NewRequest("GET", "/test", nil),
		true,
	))
	t.Run("DELETE not benchIsResponseAllowed", runner(
		nil,
		httptest.NewRequest("DELETE", "/test", nil),
		false,
	))
	t.Run("POST benchIsResponseAllowed", runner(
		[]string{"POST"},
		httptest.NewRequest("POST", "/test", nil),
		true,
	))
	t.Run("GET benchIsResponseAllowed but only POSt benchIsResponseAllowed", runner(
		[]string{"POST"},
		httptest.NewRequest("GET", "/test", nil),
		false,
	))
}

func TestPathConfigs_ConfigForPath(t *testing.T) {
	t.Parallel()

	runner := func(pc PathConfigs, r *http.Request, want string) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			c := pc.ConfigForPath(r)
			if want == "" {
				assert.Nil(t, c)
				return
			}
			if c == nil {
				t.Errorf("c should not be nil! Request Path %q; want %q", r.URL.Path, want)
			} else {
				assert.Exactly(t, want, c.Scope)
			}
		}
	}
	t.Run("/catalog/product config found", runner(
		PathConfigs{
			&PathConfig{
				Scope: "/catalog/product",
			},
			&PathConfig{
				Scope: "/checkout/cart",
			},
		},
		httptest.NewRequest("GET", "/catalog/product", nil),
		"/catalog/product",
	))
	t.Run("/catalog/product/view?a=b config found", runner(
		PathConfigs{
			&PathConfig{
				Scope: "/catalog/product",
			},
			&PathConfig{
				Scope: "/checkout/cart",
			},
		},
		httptest.NewRequest("GET", "/catalog/product/view?a=b", nil),
		"/catalog/product",
	))
	t.Run("/checkout/cart config found", runner(
		PathConfigs{
			&PathConfig{
				Scope: "/catalog/product",
			},
			&PathConfig{
				Scope: "/checkout/cart",
			},
		},
		httptest.NewRequest("GET", "/checkout/cart", nil),
		"/checkout/cart",
	))
	t.Run("/ no ESI config found, path does not match", runner(
		PathConfigs{
			&PathConfig{
				Scope: "/catalog/product",
			},
			&PathConfig{
				Scope: "/checkout/cart",
			},
		},
		httptest.NewRequest("GET", "/", nil),
		"",
	))
	t.Run("/ config found in /", runner(
		PathConfigs{
			&PathConfig{
				Scope: "/catalog/product",
			},
			&PathConfig{
				Scope: "/",
			},
		},
		httptest.NewRequest("GET", "/checkout/cart", nil),
		"/",
	))
	t.Run("/checkout/cart config found in /", runner(
		PathConfigs{
			&PathConfig{
				Scope: "/catalog/product",
			},
			&PathConfig{
				Scope: "/",
			},
		},
		httptest.NewRequest("GET", "/checkout/cart", nil),
		"/",
	))
}

func TestIsResponseAllowed(t *testing.T) {
	t.Run("HTML benchIsResponseAllowed", func(t *testing.T) {
		assert.True(t, isResponseAllowed([]byte("\r\n<html>...")))
	})
	t.Run("MP4 not benchIsResponseAllowed", func(t *testing.T) {
		assert.False(t, isResponseAllowed([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom<\x06t\xbfmdat")))
	})
	t.Run("GIF not benchIsResponseAllowed", func(t *testing.T) {
		assert.False(t, isResponseAllowed([]byte("GIF89a")))
	})
	t.Run("XML benchIsResponseAllowed", func(t *testing.T) {
		assert.True(t, isResponseAllowed([]byte("\n<?xml!")))
	})
}

var benchIsResponseAllowed bool

// BenchmarkIsResponseAllowed/Detect_binary-4         	 3000000	       462 ns/op	       0 B/op	       0 allocs/op
// BenchmarkIsResponseAllowed/Detect_html-4           	50000000	        37.3 ns/op	       0 B/op	       0 allocs/op
func BenchmarkIsResponseAllowed(b *testing.B) {
	mp4 := []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom<\x06t\xbfmdat")
	html := []byte(`<!DOCTYPE html>  <html lang="en-US"><head>     <title>Caddy Tag Tag Demo</title> </head><body> <h1> Caddy Tag Tag Demo Page</h1> <table>    <tbody>    <tr>        <th> Name</th>        <th>Output</th>    </tr>    <tr>        <td>Should now a cart from ms_cart_tiny.html</td>        <td>            <esi:include src="http://127.0.0.1:3017/ms_cart_tiny.html"/>        </td>    </tr>    </tbody></table></body></html>`)

	b.Run("Detect binary", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchIsResponseAllowed = isResponseAllowed(mp4)
		}
		if benchIsResponseAllowed {
			b.Fatal("MP4 not benchIsResponseAllowed but received true")
		}
	})

	b.Run("Detect html", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			benchIsResponseAllowed = isResponseAllowed(html)
		}
		if !benchIsResponseAllowed {
			b.Fatal("HTML benchIsResponseAllowed but received false")
		}
	})
}
