package esitag_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"bytes"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/stretchr/testify/assert"
)

func mustOpenFile(file string) *os.File {
	f, err := os.Open("testdata/" + file)
	if err != nil {
		panic(fmt.Sprintf("%s => %s", file, err))
	}
	return f
}

func strReader(s string) io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(s))
}

var testRunner = func(rc io.ReadCloser, wantTags esitag.Entities, wantErr string) func(*testing.T) {
	return func(t *testing.T) {
		defer rc.Close()
		haveTags, err := esitag.Parse(rc)
		if wantErr != "" {
			assert.Nil(t, haveTags)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), wantErr)
			return
		}
		assert.NoError(t, err)
		if have, want := len(haveTags), len(wantTags); have != want {
			t.Errorf("esitag.ESITags Count does not match: Have: %v Want: %v", have, want)
		}

		for i, tg := range wantTags {
			assert.Exactly(t, string(tg.RawTag), string(haveTags[i].RawTag))
			assert.Exactly(t, tg.TagStart, haveTags[i].TagStart)
			assert.Exactly(t, tg.TagEnd, haveTags[i].TagEnd)
		}
	}
}

// page3Results used in test and in benchmark; relates to file testdata/page3.html
var page3Results = esitag.Entities{
	&esitag.Entity{
		RawTag:   []byte(`include src="https://micr1.service/customer/account" timeout="18ms" onerror="accountNotAvailable.html"`),
		TagStart: 2009,
		TagEnd:   2118,
	},
	&esitag.Entity{
		RawTag:   []byte(`include src="https://micr2.service/checkout/cart" timeout="19ms" onerror="nocart.html" forwardheaders="Cookie,Accept-Language,Authorization"`),
		TagStart: 4043,
		TagEnd:   4190,
	},
	&esitag.Entity{
		RawTag:   []byte("include src=\"https://micr3.service/page/lastviewed\" timeout=\"20ms\" onerror=\"nofooter.html\" forwardheaders=\"Cookie,Accept-Language,Authorization\""),
		TagStart: 4455,
		TagEnd:   4606,
	},
}

func TestParseESITags_File(t *testing.T) {
	t.Run("Page0", testRunner(
		mustOpenFile("page0.html"),
		esitag.Entities{
			&esitag.Entity{
				RawTag:   []byte("include   src=\"https://micro.service/esi/foo\"\n                                            "),
				TagStart: 196,
				TagEnd:   293,
			},
		},
		"",
	))
	t.Run("Page1", testRunner(
		mustOpenFile("page1.html"),
		esitag.Entities{
			&esitag.Entity{
				RawTag:   []byte("include src=\"https://micro.service/esi/foo\" timeout=\"8ms\" onerror=\"mylocalFile.html\""),
				TagStart: 20644,
				TagEnd:   20735,
			},
		},
		"",
	))
	t.Run("Page2", testRunner(
		mustOpenFile("page2.html"),
		esitag.Entities{
			&esitag.Entity{
				RawTag:   []byte("include src=\"https://micro.service/customer/account\" timeout=\"8ms\" onerror=\"accountNotAvailable.html\""),
				TagStart: 6280,
				TagEnd:   6388,
			},
			&esitag.Entity{
				RawTag:   []byte(`include src="https://micro.service/checkout/cart" timeout="9ms" onerror="nocart.html" forwardheaders="Cookie,Accept-Language,Authorization"`),
				TagStart: 7104,
				TagEnd:   7250,
			},
		},
		"",
	))
	t.Run("Page3", testRunner(
		mustOpenFile("page3.html"),
		page3Results,
		"",
	))
}

var benchmarkParseESITags esitag.Entities

// BenchmarkParseESITags-4   	   50000	     32894 ns/op	 139.99 MB/s	    9392 B/op	      12 allocs/op
// BenchmarkParseESITags-4   	   50000	     31768 ns/op	 144.95 MB/s	    5137 B/op	       9 allocs/op <= sync.Pool Finder
// BenchmarkParseESITags-4   	   50000	     30989 ns/op	 148.60 MB/s	    1041 B/op	       8 allocs/op <= additional sync.Pool Scanner
func BenchmarkParseESITags(b *testing.B) {
	f := mustOpenFile("page3.html")
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(fi.Size())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := f.Seek(0, 0); err != nil {
			b.Fatal(err)
		}
		var err error
		benchmarkParseESITags, err = esitag.Parse(f)
		if err != nil {
			b.Fatal(err)
		}

		if have, want := len(benchmarkParseESITags), len(page3Results); have != want {
			b.Fatalf("esitag.ESITags Count does not match: Have: %v Want: %v", have, want)
		}
	}
	for i, tg := range page3Results {
		if !bytes.Equal(tg.RawTag, benchmarkParseESITags[i].RawTag) {
			b.Errorf("Tag mismatch:want: %q\nhave: %q\n", tg.RawTag, benchmarkParseESITags[i].RawTag)
		}
	}
}

func TestParseESITags_String(t *testing.T) {
	t.Run("Empty", testRunner(
		strReader(``),
		nil,
		"",
	))
	t.Run("Null Bytes", testRunner(
		strReader("x \x00 <i>x</i>          \x00<esi:include\x00 src=\"https:...\" />\x00"),
		esitag.Entities{
			&esitag.Entity{
				RawTag:   []byte("include\x00 src=\"https:...\" "),
				TagStart: 23,
				TagEnd:   55,
			},
		},
		"",
	))
	t.Run("Missing EndTag", testRunner(
		strReader(`<esi:include src="..." <b>`),
		nil,
		"",
		//"[caddyesi] Opening close tag mismatch!\n\"<esi:include src=\\\"...\\\" <b>\"\n",
	))
	t.Run("Multitags in Buffer", testRunner(
		strReader(`abcdefg<esi:include src="url1"/>u p<esi:include src="url2" />k`),
		esitag.Entities{
			&esitag.Entity{
				RawTag:   []byte("include src=\"url1\""),
				TagStart: 7,
				TagEnd:   32,
			},
			&esitag.Entity{
				RawTag:   []byte("include src=\"url2\" "),
				TagStart: 36,
				TagEnd:   62,
			},
		},
		"",
	))
}
