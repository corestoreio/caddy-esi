package esitag_test

import (
	"testing"
	"text/template"
	"time"

	"bytes"
	"os"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		t.Fatalf("%+v", err)
	}
	assert.Exactly(t, time.Millisecond*9, et.TTL)
	assert.Len(t, et.Resources.Items, 2)
	assert.Exactly(t, `resource_tpl`, et.Resources.Items[0].URLTemplate.ParseName)
	assert.Exactly(t, `resource_tpl`, et.Resources.Items[1].URLTemplate.ParseName)

	assert.Exactly(t, 0, et.Resources.Items[0].Index)
	assert.Exactly(t, 1, et.Resources.Items[1].Index)

	assert.Exactly(t, `https://micro1.service/checkout/cart/{{ .r.Header.Get "User-Agent" }}`, et.Resources.Items[0].URL)
	assert.Exactly(t, `https://micro2.service/checkout/cart/{{ .r.Header.Get "User-Agent" }}`, et.Resources.Items[1].URL)
}

func TestEntity_ParseRaw_Key_Template(t *testing.T) {
	t.Parallel()

	et := &esitag.Entity{
		RawTag: []byte(`include
			src="redisAWS1"
			key='checkout_cart_{{ .r.Header.Get "User-Agent" }}'
			src="redisAWS2"
			timeout="40ms"`),
	}
	if err := et.ParseRaw(); err != nil {
		t.Fatal(err)
	}
	assert.Exactly(t, time.Millisecond*40, et.Timeout)

	assert.Len(t, et.Resources.Items, 2)
	assert.Empty(t, et.Key)
	assert.Exactly(t, `key_tpl`, et.KeyTemplate.ParseName)

	assert.Exactly(t, `redisAWS1`, et.Resources.Items[0].URL)
	assert.Nil(t, et.Resources.Items[0].URLTemplate)
	assert.False(t, et.Resources.Items[0].IsURL)
	assert.Exactly(t, 0, et.Resources.Items[0].Index)

	assert.Exactly(t, `redisAWS2`, et.Resources.Items[1].URL)
	assert.False(t, et.Resources.Items[1].IsURL)
	assert.Exactly(t, 1, et.Resources.Items[1].Index)
	assert.Nil(t, et.Resources.Items[1].URLTemplate)
}

func TestESITag_ParseRaw(t *testing.T) {
	t.Parallel()

	runner := func(rawTag []byte, wantErrBhf errors.BehaviourFunc, wantET *esitag.Entity) func(*testing.T) {
		return func(t *testing.T) {
			if wantET != nil {
				wantET.RawTag = rawTag
			}
			haveET := &esitag.Entity{
				RawTag: rawTag,
			}

			if haveErr := haveET.ParseRaw(); wantErrBhf != nil {
				assert.True(t, wantErrBhf(haveErr), "%+v", haveErr)
				return
			} else {
				require.NoError(t, haveErr)
			}

			assert.Exactly(t, wantET.DataTag, haveET.DataTag, "Tag")
			assert.Exactly(t, len(wantET.Resources.Items), len(haveET.Resources.Items), "Len Resource Items")
			assert.Exactly(t, wantET.Resources.MaxBodySize, haveET.Resources.MaxBodySize)

			if wantET.Resources.Items != nil {
				for i, ri := range wantET.Resources.Items {
					haveRI := haveET.Resources.Items[i]
					assert.Exactly(t, ri.Index, haveRI.Index, "Resource Item Index")
					assert.Exactly(t, ri.IsURL, haveRI.IsURL, "Resource Item IsURL")
					assert.Exactly(t, ri.URL, haveRI.URL, "Resource Item URL")
				}
			}

			assert.Exactly(t, wantET.RawTag, haveET.RawTag, "RawTag")
			assert.Exactly(t, wantET.TTL, haveET.TTL, "TTL")
			assert.Exactly(t, wantET.Timeout, haveET.Timeout, "Timeout")
			assert.Exactly(t, wantET.OnError, haveET.OnError, "OnError")
			assert.Exactly(t, wantET.ForwardHeaders, haveET.ForwardHeaders, "ForwardHeaders")
			assert.Exactly(t, wantET.ForwardHeadersAll, haveET.ForwardHeadersAll, "ForwardHeadersAll")
			assert.Exactly(t, wantET.ForwardHeadersAll, haveET.ForwardHeadersAll, "ForwardHeadersAll")
			assert.Exactly(t, wantET.ReturnHeaders, haveET.ReturnHeaders, "ReturnHeaders")
			assert.Exactly(t, wantET.ReturnHeadersAll, haveET.ReturnHeadersAll, "ReturnHeadersAll")
			assert.Exactly(t, wantET.Key, haveET.Key, "Key")
			if wantET.KeyTemplate != nil {
				assert.Exactly(t,
					wantET.KeyTemplate.ParseName,
					haveET.KeyTemplate.ParseName, "KeyTemplate.ParseName")
			}

		}
	}

	t.Run("1x src, timeout, onerror, forwardheaders", runner(
		[]byte(`include src="https://micro.service/checkout/cart" timeout="9ms" onerror="nocart.html" forwardheaders="Cookie , Accept-Language, Authorization"`),
		nil,
		&esitag.Entity{
			Resources: esitag.Resources{
				Items: []*esitag.Resource{
					{URL: "https://micro.service/checkout/cart", IsURL: true},
				},
			},
			Timeout:        time.Millisecond * 9,
			OnError:        "nocart.html",
			ForwardHeaders: []string{"Cookie", "Accept-Language", "Authorization"},
		},
	))

	t.Run("2x src, timeout, onerror, forwardheaders", runner(
		[]byte(`include src="https://micro1.service/checkout/cart" src="https://micro2.service/checkout/cart" ttl="9ms"  returnheaders="Cookie , Accept-Language, Authorization"`),
		nil,
		&esitag.Entity{
			Resources: esitag.Resources{
				Items: []*esitag.Resource{
					{URL: "https://micro1.service/checkout/cart", IsURL: true, Index: 0},
					{URL: "https://micro2.service/checkout/cart", IsURL: true, Index: 1},
				},
			},
			TTL:           time.Millisecond * 9,
			ReturnHeaders: []string{"Cookie", "Accept-Language", "Authorization"},
		},
	))

	t.Run("at least one source attribute is required", runner(
		[]byte(`include key="product_234234" returnheaders=" all  " forwardheaders=" all  "`),
		errors.IsEmpty,
		nil,
	))

	t.Run("white spaces in returnheaders and forwardheaders", runner(
		[]byte(`include key="product_234234" returnheaders=" all  " forwardheaders=" all  " src="awsRedis1"`),
		nil,
		&esitag.Entity{
			Resources: esitag.Resources{
				Items: []*esitag.Resource{
					{URL: "awsRedis1", IsURL: false, Index: 0},
				},
			},
			Key:               "product_234234",
			ReturnHeadersAll:  true,
			ForwardHeadersAll: true,
		},
	))

	t.Run("resource not an URL but an alias to KV storage", runner(
		[]byte(`include key="product_4711" returnheaders='all' forwardheaders="all	" src="awsRedis3"`),
		nil,
		&esitag.Entity{
			Resources: esitag.Resources{
				Items: []*esitag.Resource{
					{URL: "awsRedis3", IsURL: false, Index: 0},
				},
			},
			Key:               "product_4711",
			ReturnHeadersAll:  true,
			ForwardHeadersAll: true,
		},
	))

	t.Run("key as template with single quotes", runner(
		[]byte(`include key='product_234234_{{ .r.Header.Get "myHeaderKey" }}' src="awsRedis2"  returnheaders=" all  " forwardheaders=" all  "`),
		nil,
		&esitag.Entity{
			Resources: esitag.Resources{
				Items: []*esitag.Resource{
					{URL: "awsRedis2", IsURL: false, Index: 0},
				},
			},
			KeyTemplate:       template.Must(template.New("key_tpl").Parse("unimportant")),
			ReturnHeadersAll:  true,
			ForwardHeadersAll: true,
		},
	))

	t.Run("ignore attribute starting with x", runner(
		[]byte(`include xkey='product_234234_{{ .r.Header.Get "myHeaderKey" }}' src="awsRedis2"  returnheaders=" all  " forwardheaders=" all  "`),
		nil,
		&esitag.Entity{
			Resources: esitag.Resources{
				Items: []*esitag.Resource{
					{URL: "awsRedis2", IsURL: false, Index: 0},
				},
			},
			ReturnHeadersAll:  true,
			ForwardHeadersAll: true,
		},
	))

	t.Run("show not supported unknown attribute", runner(
		[]byte(`include ykey='product_234234_{{ .r.Header.Get "myHeaderKey" }}' src="awsRedis2"  returnheaders=" all  " forwardheaders=" all  "`),
		errors.IsNotSupported,
		nil,
	))

	t.Run("timeout parsing failed", runner(
		[]byte(`include timeout="9a"`),
		errors.IsNotValid,
		nil,
	))

	t.Run("ttl parsing failed", runner(
		[]byte(`include ttl="8a"`),
		errors.IsNotValid,
		nil,
	))

	t.Run("key template parsing failed", runner(
		[]byte(`include key='product_234234_{{ .r.Header.Get 'myHeaderKey" }}' returnheaders=" all  " forwardheaders=" all  "`),
		errors.IsNotSupported,
		nil,
	))

	t.Run("key only one quotation mark and fails", runner(
		[]byte(`include key='`),
		errors.IsEmpty,
		nil,
	))

	t.Run("failed to parse src", runner(
		[]byte(`include src='https://catalog.corestore.io/product={{ $r. }'`),
		errors.IsFatal,
		nil,
	))

	t.Run("failed to balanced pairs", runner(
		[]byte(`src='https://catalog.corestore.io/product='`),
		errors.IsNotValid,
		nil,
	))

	t.Run("failed to parse condition", runner(
		[]byte(`include condition='{{ $r. }'`),
		errors.IsFatal,
		nil,
	))

}

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

func TestSplitAttributes(t *testing.T) {

	runner := func(in string, want []string, wantErrBhf errors.BehaviourFunc) func(*testing.T) {
		return func(t *testing.T) {
			have, haveErr := esitag.SplitAttributes(in)
			if wantErrBhf != nil {
				assert.True(t, wantErrBhf(haveErr), "%+v", haveErr)
			} else if haveErr != nil {
				t.Errorf("Error not expected: %+v", haveErr)
			}
			assert.Exactly(t, want, have)
		}
	}

	t.Run("Split without errors", runner(
		`include
	 src='https://micro1.service/product/id={{ .r.Header.Get "myHeaderKey" }}'
	 	src="https://micro2.service/checkout/cart" ttl=" 19ms"  timeout="9ms" onerror='nocart.html'
	forwardheaders=" Cookie , Accept-Language, Authorization" returnheaders="Set-Cookie , Authorization "`,
		[]string{
			"src", "https://micro1.service/product/id={{ .r.Header.Get \"myHeaderKey\" }}",
			"src", "https://micro2.service/checkout/cart",
			"ttl", "19ms",
			"timeout", "9ms",
			"onerror", "nocart.html",
			"forwardheaders", "Cookie , Accept-Language, Authorization",
			"returnheaders", "Set-Cookie , Authorization",
		},
		nil,
	))

	t.Run("Split imbalanced", runner(
		`src="https://micro2.service/checkout/cart" ttl=" 19ms"`,
		nil,
		errors.IsNotValid,
	))

	t.Run("Unicode correct", runner(
		`include src="https://.Ø/checkout/cart" ttl="€"`,
		[]string{"src", "https://\uf8ff.Ø/checkout/cart", "ttl", "€"},
		nil,
	))

	t.Run("Whitespace", runner(
		` `,
		[]string{},
		nil,
	))

	t.Run("Empty", runner(
		``,
		[]string{},
		nil,
	))

}

func TestDataTags_InjectContent(t *testing.T) {
	t.Parallel()

	runner := func(fileName string, content [][]byte) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			page3F, err := os.Open(fileName)
			if err != nil {
				t.Fatal(err)
			}
			ets, err := esitag.Parse(page3F)
			if err != nil {
				t.Fatal(err)
			}

			var tags = make(esitag.DataTags, len(ets))
			for k := 0; k < len(ets); k++ {
				ets[k].DataTag.Data = content[k]
				tags = append(tags, ets[k].DataTag)
			}

			w := new(bytes.Buffer)
			if _, err := page3F.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			if err := tags.InjectContent(page3F, w); err != nil {
				t.Fatalf("%+v", err)
			}

			for k := 0; k < len(content); k++ {
				assert.Contains(t, w.String(), string(content[k]))
				if have, want := bytes.Count(w.Bytes(), content[k]), 1; have != want {
					t.Errorf("Have: %d Want: %d", have, want)
				}

			}
		}
	}
	t.Run("Page1", runner("testdata/page1.html",
		[][]byte{
			[]byte(`<p>Hello Jonathan Gopher. You're logged in.</p>`),
		},
	))
	t.Run("Page2", runner("testdata/page2.html",
		[][]byte{
			[]byte(`<p>Hello John Gopher. You're logged in.</p>`),
			[]byte(`<h1>You have 4 items in your shopping cart.</h1>`),
		},
	))
	t.Run("Page3", runner("testdata/page3.html",
		[][]byte{
			[]byte(`<p>This microservice generates content one. </p>`),
			[]byte(`<h1>This microservice generates content two. </h1>`),
			[]byte(`<script> alert('This microservice generates content three. ');</script>`),
		},
	))
	t.Run("Page4", runner("testdata/page4.html",
		[][]byte{
			[]byte(`<p>This microservice generates content one. </p>`),
			[]byte(`<h1>This microservice generates content two. @</h1>`),
			[]byte(`<h1>This microservice generates content three. €</h1>`),
			[]byte(`<h1>This microservice generates content four. 4</h1>`),
			[]byte(`<h1>This microservice generates content five. 5</h1>`),
		},
	))

}

func BenchmarkDataTags_InjectContent(b *testing.B) {

	runner := func(fileName string, content [][]byte) func(*testing.B) {
		return func(b *testing.B) {

			page3F, err := os.Open(fileName)
			if err != nil {
				b.Fatal(err)
			}

			ets, err := esitag.Parse(page3F)
			if err != nil {
				b.Fatal(err)
			}

			var tags = make(esitag.DataTags, len(ets))
			for k := 0; k < len(ets); k++ {
				ets[k].DataTag.Data = content[k]
				tags = append(tags, ets[k].DataTag)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				w := new(bytes.Buffer)
				if _, err := page3F.Seek(0, 0); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()

				if err := tags.InjectContent(page3F, w); err != nil {
					b.Fatalf("%+v", err)
				}

				b.StopTimer()
				for k := 0; k < len(content); k++ {
					assert.Contains(b, w.String(), string(content[k]))
				}
				b.StartTimer()
			}
		}
	}
	b.Run("Page1", runner("testdata/page1.html",
		[][]byte{
			[]byte(`<p>Hello Jonathan Gopher. You're logged in.</p>`),
		},
	))
	b.Run("Page2", runner("testdata/page2.html",
		[][]byte{
			[]byte(`<p>Hello John Gopher. You're logged in.</p>`),
			[]byte(`<h1>You have 4 items in your shopping cart.</h1>`),
		},
	))
	b.Run("Page3", runner("testdata/page3.html",
		[][]byte{
			[]byte(`<p>This microservice generates content one. </p>`),
			[]byte(`<h1>This microservice generates content two. </h1>`),
			[]byte(`<script> alert('This microservice generates content three. ');</script>`),
		},
	))
	b.Run("Page4", runner("testdata/page4.html",
		[][]byte{
			[]byte(`<p>This microservice generates content one. </p>`),
			[]byte(`<h1>This microservice generates content two. @</h1>`),
			[]byte(`<h1>This microservice generates content three. €</h1>`),
			[]byte(`<h1>This microservice generates content four. 4</h1>`),
			[]byte(`<h1>This microservice generates content five. 5</h1>`),
		},
	))
}

func TestEntities_QueryResources(t *testing.T) {
	t.Skip("TODO")
}
