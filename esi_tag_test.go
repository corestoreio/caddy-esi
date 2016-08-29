package esi

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func mustOpenFile(file string) io.ReadCloser {
	f, err := os.Open("testdata/" + file)
	if err != nil {
		panic(fmt.Sprintf("%s => %s", file, err))
	}
	return f
}

func strReader(s string) io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(s))
}

var testRunner = func(rc io.ReadCloser, wantTags ESITags, wantErr string) func(*testing.T) {
	return func(t *testing.T) {
		defer rc.Close()
		haveTags, err := ParseESITags(rc)
		if wantErr != "" {
			assert.Nil(t, haveTags)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), wantErr)
			return
		}
		assert.NoError(t, err)
		if have, want := len(haveTags), len(wantTags); have != want {
			t.Errorf("ESITags Count does not match: Have: %v Want: %v", have, want)
		}
		if len(wantTags) <= len(haveTags) {
			for i, tg := range wantTags {
				assert.Exactly(t, string(tg.RawTag), string(haveTags[i].RawTag))
			}
		}
		if len(haveTags) <= len(wantTags) {
			for i, tg := range haveTags {
				assert.Exactly(t, string(wantTags[i].RawTag), string(tg.RawTag))
			}
		}
	}
}

func TestParseESITags_File(t *testing.T) {
	t.Run("Page0", testRunner(
		mustOpenFile("page0.html"),
		ESITags{
			&ESITag{
				RawTag: []byte("<esi:include   src=\"https://micro.service/esi/foo\"\n                                            />"),
			},
		},
		"",
	))
	t.Run("Page1", testRunner(
		mustOpenFile("page1.html"),
		ESITags{
			&ESITag{
				RawTag: []byte("<esi:include src=\"https://micro.service/esi/foo\" timeout=\"8ms\" onerror=\"mylocalFile.html\"/>"),
			},
		},
		"",
	))
	t.Run("Page2", testRunner(
		mustOpenFile("page2.html"),
		ESITags{
			&ESITag{
				RawTag: []byte("<esi:include src=\"https://micro.service/customer/account\" timeout=\"8ms\" onerror=\"accountNotAvailable.html\"/>"),
			},
			&ESITag{
				RawTag: []byte(`<esi:include src="https://micro.service/checkout/cart" timeout="9ms" onerror="nocart.html" forwardheaders="Cookie,Accept-Language,Authorization"/>`),
			},
		},
		"",
	))
	t.Run("Page3 Buffer Lookahead", testRunner(
		mustOpenFile("page3.html"),
		ESITags{
			&ESITag{
				RawTag: []byte(`<esi:include src="https://micr1.service/customer/account" timeout="18ms" onerror="accountNotAvailable.html"/>`),
			},
			&ESITag{
				RawTag: []byte(`<esi:include src="https://micr2.service/checkout/cart" timeout="19ms" onerror="nocart.html" forwardheaders="Cookie,Accept-Language,Authorization"/>`),
			},
		},
		"",
	))
}

func TestParseESITags_String(t *testing.T) {
	t.Run("Empty", testRunner(
		strReader(``),
		nil,
		"",
	))
	t.Run("Null Bytes", testRunner(
		strReader("x \x00 <i>x</i>          \x00<esi:include\x00 src=\"https:...\" />\x00"),
		ESITags{
			&ESITag{
				RawTag: []byte("<esi:include\x00 src=\"https:...\" />"),
			},
		},
		"",
	))
	t.Run("Missing EndTag", testRunner(
		strReader(`<esi:include src="..." <b>`),
		nil,
		"[caddyesi] Opening close tag mismatch!\n\"<esi:include src=\\\"...\\\" <b>\"\n",
	))
	t.Run("Multitags in Buffer", testRunner(
		strReader("abcdefg<esi:include src=\"url1\"/>u\np<esi:include src=\"url2\" />k"),
		ESITags{
			&ESITag{
				RawTag: []byte("<esi:include src=\"url1\"/>"),
			},
			&ESITag{
				RawTag: []byte("<esi:include src=\"url2\" />"),
			},
		},
		"",
	))
}
