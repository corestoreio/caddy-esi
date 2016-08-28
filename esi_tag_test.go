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
			t.Fatalf("ESITags Count does not match: Have: %v Want: %v", have, want)
		}
		for i, tg := range wantTags {
			assert.Exactly(t, string(tg.RawTag), string(haveTags[i].RawTag))
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
				RawTag: []byte("<esi:include srsrc=\"https://micro.service/checkout/cat\" timeout=\"9ms\" onerror=\"nonocart.html\" forwardheaders=\"Cookie,Accept-Language,Authorization\"/>"),
			},
		},
		"",
	))
}

func TestParseESITags_String(t *testing.T) {
	t.Run("Line_80", testRunner(
		strReader(``),
		nil,
		"",
	))
	t.Run("Line_85", testRunner(
		strReader("x \x00 <i>x</i>          <esi:include\x00 src=\"https:...\" />\x00"),
		ESITags{
			&ESITag{
				RawTag: []byte("<esi:include\x00\x00 src=\"https:...\" />"),
			},
		},
		"",
	))
	t.Run("Line_73", testRunner(
		strReader(`<esi:include src="..." <b>`),
		nil,
		"[caddyesi] Opening close tag mismatch!\n\"<esi:include src=\\\"...\\\" <b>\"\n",
	))
}
