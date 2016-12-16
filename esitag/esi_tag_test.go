package esitag_test

import (
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/stretchr/testify/assert"
)

func TestESITag_ParseRaw(t *testing.T) {
	tests := []struct {
		raw     []byte
		wantErr string
		wantTag *esitag.Entity
	}{
		{
			[]byte(`include src="https://micro.service/checkout/cart" timeout="9ms" onerror="nocart.html" forwardheaders="Cookie , Accept-Language, Authorization"`),
			"",
			&esitag.Entity{
				Resources: []esitag.Resource{
					{URL: "https://micro.service/checkout/cart"},
				},
				Timeout:        time.Millisecond * 9,
				OnError:        "nocart.html",
				ForwardHeaders: []string{"Cookie", "Accept-Language", "Authorization"},
			},
		},
		{
			[]byte(`include src="https://micro1.service/checkout/cart" src="https://micro2.service/checkout/cart" ttl="9ms"  returnheaders="Cookie , Accept-Language, Authorization"`),
			"",
			&esitag.Entity{
				Resources: []esitag.Resource{
					{URL: "https://micro1.service/checkout/cart"},
					{URL: "https://micro2.service/checkout/cart"},
				},
				TTL:           time.Millisecond * 9,
				ReturnHeaders: []string{"Cookie", "Accept-Language", "Authorization"},
			},
		},
		{
			[]byte(`include key="product_234234" returnheaders=" all  " forwardheaders=" all  "`),
			"",
			&esitag.Entity{
				ResourceKey:       esitag.ResourceKey{Key: "product_234234"},
				ReturnHeadersAll:  true,
				ForwardHeadersAll: true,
			},
		},
		//{
		//	[]byte(`include key="product_234234_{{ r.Header.Get myHeaderKey }}" returnheaders=" all  " forwardheaders=" all  "`),
		//	"",
		//	&esitag.Entity{
		//		ResourceKey:       esitag.ResourceKey{Key: "product_234234"},
		//		ReturnHeadersAll:  true,
		//		ForwardHeadersAll: true,
		//	},
		//},
		{
			[]byte(`include timeout="9a"`),
			` timeout: time: unknown unit a in duration 9a => "9a"`,
			&esitag.Entity{},
		},
		{
			[]byte(`include ttl="8a"`),
			` in ttl: time: unknown unit a in duration 8a => "8a"`,
			&esitag.Entity{},
		},
	}
	for i, test := range tests {
		test.wantTag.RawTag = test.raw
		haveET := &esitag.Entity{
			RawTag: test.raw,
		}
		haveErr := haveET.ParseRaw()
		if test.wantErr != "" {
			assert.Error(t, haveErr, "Index %d", i)
			assert.Contains(t, haveErr.Error(), test.wantErr, "Index %d", i)
			continue
		}
		assert.NoError(t, haveErr)
		assert.Exactly(t, *test.wantTag, *haveET, "Index %d", i)
	}
}

// 100000	     15671 ns/op	    1810 B/op	      28 allocs/op
func BenchmarkESITag_ParseRaw_MicroServicse(b *testing.B) {
	et := &esitag.Entity{
		RawTag: []byte(`include
	 src="https://micro1.service/checkout/cart" src="https://micro2.service/checkout/cart" ttl="19ms"  timeout="9ms" onerror="nocart.html"
	forwardheaders="Cookie , Accept-Language, Authorization" returnheaders="Set-Cookie , Authorization"`),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := et.ParseRaw(); err != nil {
			b.Fatal(err)
		}
	}
	if have, want := et.OnError, "nocart.html"; have != want {
		b.Errorf("Have: %v Want: %v", have, want)
	}
}

func TestESITags_ParseKey(t *testing.T) {
	t.Error("@todo")
}

func TestESITags_ParseCondition(t *testing.T) {
	t.Error("@todo")
}

func TestESITags_ParseResource(t *testing.T) {
	t.Error("@todo")
}
