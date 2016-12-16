package esitag_test

import (
	"testing"
	"time"

	"text/template"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/stretchr/testify/assert"
)

func TestEntity_ParseRaw_Src_Template(t *testing.T) {
	t.Parallel()

	et := &esitag.Entity{
		RawTag: []byte(`include
			src='https://micro1.service/checkout/cart/{{ .r.Header.Get "User-Agent" }}'
			src='https://micro2.service/checkout/cart/{{ .r.Header.Get "User-Agent" }}'
			ttl="9ms"`),
	}
	if err := et.ParseRaw(); err != nil {
		t.Fatal(err)
	}
	assert.Exactly(t, time.Millisecond*9, et.TTL)
	assert.Len(t, et.Resources, 2)
	assert.Exactly(t, `resource_tpl`, et.Resources[0].Template.ParseName)
	assert.Exactly(t, `resource_tpl`, et.Resources[1].Template.ParseName)

	assert.Exactly(t, 0, et.Resources[0].Index)
	assert.Exactly(t, 1, et.Resources[1].Index)

	assert.Empty(t, et.Resources[1].KVNet)
	assert.Empty(t, et.Resources[1].URL)
}

func TestESITag_ParseRaw(t *testing.T) {
	t.Parallel()

	runner := func(raw []byte, wantErr string, wantTag *esitag.Entity) func(*testing.T) {
		return func(t *testing.T) {
			if wantTag != nil {
				wantTag.RawTag = raw
			}
			haveET := &esitag.Entity{
				RawTag: raw,
			}
			haveErr := haveET.ParseRaw()
			if wantErr != "" {
				assert.Error(t, haveErr)
				assert.Contains(t, haveErr.Error(), wantErr)
				return
			}
			assert.NoError(t, haveErr)

			var haveRKTpl *template.Template
			if wantTag.ResourceKey.Template != nil {
				wantTag.ResourceKey.Template = nil
				haveRKTpl = haveET.Template
				haveET.Template = nil
			}
			assert.Exactly(t, *wantTag, *haveET)

			if haveRKTpl == nil {
				return
			}
			assert.Exactly(t, `key_tpl`, haveRKTpl.ParseName)
		}
	}

	t.Run("1x src, timeout, onerror, forwardheaders", runner(
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
	))

	t.Run("2x src, timeout, onerror, forwardheaders", runner(
		[]byte(`include src="https://micro1.service/checkout/cart" src="https://micro2.service/checkout/cart" ttl="9ms"  returnheaders="Cookie , Accept-Language, Authorization"`),
		"",
		&esitag.Entity{
			Resources: []esitag.Resource{
				{URL: "https://micro1.service/checkout/cart", Index: 0},
				{URL: "https://micro2.service/checkout/cart", Index: 1},
			},
			TTL:           time.Millisecond * 9,
			ReturnHeaders: []string{"Cookie", "Accept-Language", "Authorization"},
		},
	))

	t.Run("at least one source attribute is required", runner(
		[]byte(`include key="product_234234" returnheaders=" all  " forwardheaders=" all  "`),
		"[caddyesi] ESITag.ParseRaw. src cannot be empty in Tag which requires at least one resource:",
		nil,
	))

	t.Run("white spaces in returnheaders and forwardheaders", runner(
		[]byte(`include key="product_234234" returnheaders=" all  " forwardheaders=" all  " src="httpee"`),
		"",
		&esitag.Entity{
			ResourceKey:       esitag.ResourceKey{Key: "product_234234"},
			Resources:         esitag.Resources{esitag.Resource{KVNet: "httpee"}},
			ReturnHeadersAll:  true,
			ForwardHeadersAll: true,
		},
	))

	t.Run("resource not an URL but a KVNet", runner(
		[]byte(`include key="product_234234" returnheaders=" all  " forwardheaders=" all  " src="awsRedis1"`),
		"",
		&esitag.Entity{
			ResourceKey:       esitag.ResourceKey{Key: "product_234234"},
			Resources:         esitag.Resources{esitag.Resource{KVNet: "awsRedis1"}},
			ReturnHeadersAll:  true,
			ForwardHeadersAll: true,
		},
	))

	t.Run("key as template with single quotes", runner(
		[]byte(`include key='product_234234_{{ .r.Header.Get "myHeaderKey" }}' src="awsRedis1"  returnheaders=" all  " forwardheaders=" all  "`),
		"",
		&esitag.Entity{
			Resources:         esitag.Resources{esitag.Resource{KVNet: "awsRedis1"}},
			ResourceKey:       esitag.ResourceKey{Key: "", Template: new(template.Template)},
			ReturnHeadersAll:  true,
			ForwardHeadersAll: true,
		},
	))

	t.Run("timeout parsing failed", runner(
		[]byte(`include timeout="9a"`),
		` timeout: time: unknown unit a in duration 9a => "9a"`,
		nil,
	))

	t.Run("ttl parsing failed", runner(
		[]byte(`include ttl="8a"`),
		` in ttl: time: unknown unit a in duration 8a => "8a"`,
		nil,
	))

	t.Run("key template parsing failed", runner(
		[]byte(`include key='product_234234_{{ .r.Header.Get 'myHeaderKey" }}' returnheaders=" all  " forwardheaders=" all  "`),
		`Failed to parse key "product_234234_{{ .r.Header.Get" in tag`,
		nil,
	))

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
