package esi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		r    *http.Request
		want uint64
	}{
		{httptest.NewRequest("GET", "/", nil), 0x7a6e1f1822179273},
		{httptest.NewRequest("GET", "/test", nil), 0x2e3a61d5bfffd7d2},
	}
	for i, test := range tests {
		assert.Exactly(t, test.want, requestID(test.r), "Index %d", i)
	}
}

var benchmarkRequestID uint64

// 20000000	       109 ns/op	      64 B/op	       1 allocs/op
func BenchmarkRequestID(b *testing.B) {
	r := httptest.NewRequest("GET", "/catalog/product/id/42342342/price/134.231/stock/1/camera.html", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkRequestID = requestID(r)
	}
}

func TestParseBackendUrl(t *testing.T) {
	t.Run("Redis", func(t *testing.T) {
		be, err := parseBackendUrl("redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/0")
		assert.NoError(t, err)
		_, ok := be.(*Redis)
		assert.True(t, ok, "Expecting Redis in the Backender interface")
	})
	t.Run("URL Error", func(t *testing.T) {
		be, err := parseBackendUrl("redis//localhost")
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown URL: "redis//localhost". Does not contain ://`)
	})
	t.Run("Scheme Error", func(t *testing.T) {
		be, err := parseBackendUrl("mysql://localhost")
		assert.Nil(t, be)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `Unknown URL: "mysql://localhost". No driver defined for scheme: "mysql"`)
	})

}
