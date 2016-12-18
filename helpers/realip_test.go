package helpers_test

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/stretchr/testify/assert"
)

func TestGetRealIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		r      *http.Request
		wantIP net.IP
	}{
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Real-IP", "123.123.123.123")
			return r
		}(), net.ParseIP("123.123.123.123")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("Forwarded-For", "200.100.50.3")
			return r
		}(), net.ParseIP("200.100.50.3")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Forwarded", "2002:0db8:85a3:0000:0000:8a2e:0370:7335")
			return r
		}(), net.ParseIP("2002:0db8:85a3:0000:0000:8a2e:0370:7335")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Forwarded-For", "200.100.54.4, 192.168.0.100:8080")
			return r
		}(), net.ParseIP("192.168.0.100")}, // maybe a bug because it returns the internal proxy IP address
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Cluster-Client-Ip", "127.0.0.1:8080")
			r.RemoteAddr = "200.100.54.6:8181"
			return r
		}(), net.ParseIP("200.100.54.6")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.RemoteAddr = "100.200.50.3"
			return r
		}(), net.ParseIP("100.200.50.3")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.Header.Set("X-Forwarded-For", "127.0.0.1:8080")
			r.RemoteAddr = "2002:0db8:85a3:0000:0000:8a2e:0370:7334"
			return r
		}(), net.ParseIP("2002:0db8:85a3:0000:0000:8a2e:0370:7334")},
		{func() *http.Request {
			r := httptest.NewRequest("GET", "http://gopher.go", nil)
			r.RemoteAddr = "100.200.a.3"
			return r
		}(), nil},
	}

	for i, test := range tests {
		haveIP := helpers.RealIP(test.r)
		assert.Exactly(t, test.wantIP, haveIP, "Index: %d Want %s Have %s", i, test.wantIP, haveIP)
	}
}
