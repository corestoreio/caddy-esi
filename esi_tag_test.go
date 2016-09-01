package esi

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestESITag_ParseRaw(t *testing.T) {
	tests := []struct {
		raw     []byte
		wantErr string
		wantTag *ESITag
	}{
		{
			[]byte(`include src="https://micro.service/checkout/cart" timeout="9ms" onerror="nocart.html" forwardheaders="Cookie,Accept-Language,Authorization"`),
			"",
			&ESITag{
				Sources: []fmt.Stringer{
					source("https://micro.service/checkout/cart"),
				},
				Timeout:        time.Millisecond * 9,
				OnError:        "nocart.html",
				ForwardHeaders: []string{"Cookie", "Accept-Language", "Authorization"},
			},
		},
	}
	for i, test := range tests {
		test.wantTag.RawTag = test.raw
		haveET := &ESITag{
			RawTag:   test.raw,
			TagStart: 0,
			TagEnd:   len(test.raw),
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
