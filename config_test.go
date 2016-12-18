package caddyesi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SchumacherFM/caddyesi/esiredis"
	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/stretchr/testify/assert"
)

const weirdLongUrl = `https://app.usunu.com/-/login?u=https%3A%2F%2Fapp.usunu.com%2F0%2Fsearch%2F2385944396396%2F81453167684176&e=emailaddress%40gmail.com&passive=1`

func TestPathConfig_RequestID(t *testing.T) {
	// t.Parallel()

	runner := func(requestIDSource []string, r *http.Request, wantSum uint64) func(*testing.T) {
		return func(t *testing.T) {
			//t.Parallel()

			pc := NewPathConfig()
			pc.RequestIDSource = requestIDSource

			if have, want := pc.requestID(r), wantSum; have != want {
				t.Errorf("Have: %x Want: %x", have, want)
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
			r.Header.Set(helpers.XClusterClientIP, "127.0.0.2")
			return r
		}(),
		0x8b7d8dd0ed3fb96d, // hash of the byte slice of cluster client IP
	))
	t.Run("scheme", runner(
		[]string{"scheme"},
		httptest.NewRequest("GET", "https://caddyserver.com/test", nil),
		0x909acbb899ed37e6,
	))

	t.Run("host", runner(
		[]string{"host"},
		httptest.NewRequest("GET", weirdLongUrl, nil),
		0x16a46f35c998d63d,
	))
	t.Run("path", runner(
		[]string{"path"},
		httptest.NewRequest("GET", weirdLongUrl, nil),
		0x163de6d2f60202bc, // path is: /-/login
	))
	t.Run("rawpath", runner(
		[]string{"rawpath"},
		httptest.NewRequest("GET", weirdLongUrl, nil),
		0xb4b967239b2b0817, // rawpath is: app.usunu.com/-/login
	))
	t.Run("rawquery", runner(
		[]string{"rawquery"},
		httptest.NewRequest("GET", weirdLongUrl, nil),
		0xb08e9c9fd24079b4, // rawquery is: u=https%3A%2F%2Fapp.usunu.com%2F0%2Fsearch%2F2385944396396%2F81453167684176&e=emailaddress%40gmail.com&passive=1
	))
	t.Run("url", runner(
		[]string{"url"},
		httptest.NewRequest("GET", weirdLongUrl, nil),
		0x6c7360d1c2978e84, // full url
	))

}

var benchmarkRequestID uint64

func BenchmarkRequestID(b *testing.B) {
	r := httptest.NewRequest("GET", "/catalog/product/id/42342342/price/134.231/stock/1/camera.html", nil)
	pc := NewPathConfig()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkRequestID = pc.requestID(r)
	}
}

func BenchmarkRequestID_FullURL(b *testing.B) {
	r := httptest.NewRequest("GET", weirdLongUrl, nil)

	pc := NewPathConfig()
	pc.RequestIDSource = []string{"url"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkRequestID = pc.requestID(r)
	}
}

func BenchmarkRequestID_Cookie(b *testing.B) {
	r := httptest.NewRequest("GET", weirdLongUrl, nil)
	r.AddCookie(&http.Cookie{Name: "xtestKeks", Value: "xVal"})

	pc := NewPathConfig()
	pc.RequestIDSource = []string{"cookie-xtestKeks"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkRequestID = pc.requestID(r)
	}
}

func TestParseBackendUrl(t *testing.T) {
	t.Run("Redis", func(t *testing.T) {
		be, err := newKVFetcher("redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0")
		assert.NoError(t, err)
		_, ok := be.(*esiredis.Redis)
		assert.True(t, ok, "Expecting Redis in the Backender interface")
	})
	t.Run("URL Error", func(t *testing.T) {
		be, err := newKVFetcher("redis//localhost")
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown URL: "redis//localhost". Does not contain ://`)
	})
	t.Run("Scheme Error", func(t *testing.T) {
		be, err := newKVFetcher("mysql://localhost")
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown URL: "mysql://localhost". No driver defined for scheme: "mysql"`)
	})

}
