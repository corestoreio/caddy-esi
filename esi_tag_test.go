package esi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseESITags(t *testing.T) {

	var testRunner = func(file string, wantTags ESITags, wantErr string) func(*testing.T) {
		return func(t *testing.T) {
			f, err := os.Open("testdata/" + file)
			if err != nil {
				t.Fatalf("%s => %s", file, err)
			}
			defer f.Close()

			haveTags, err := ParseESITags(f)
			if wantErr != "" {
				assert.Nil(t, haveTags)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), wantErr)
				return
			}
			assert.NoError(t, err)
			if have, want := len(haveTags), len(wantTags); have != want {
				t.Fatalf("ESITags Count does not match: Have: %v Want: %v; File: %q", have, want, file)
			}
			for i, tg := range wantTags {
				assert.Exactly(t, string(tg.RawTag), string(haveTags[i].RawTag))
			}
		}
	}
	t.Run("Page1", testRunner(
		"page0.html",
		ESITags{
			&ESITag{
				RawTag: []byte("<esi:include   src=\"https://micro.service/esi/foo\"\n                                            />"),
			},
		},
		"",
	))
}
