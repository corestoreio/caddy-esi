package esitag_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

var _ esitag.ResourceRequestFunc = esitag.FetchHTTP

func TestFetchHTTP(t *testing.T) {
	// All tests modifying TestClient cannot be run in parallel.

	t.Run("LimitedReader", func(t *testing.T) {
		esitag.TestClient = &http.Client{
			Transport: esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", nil),
		}

		content, err := esitag.FetchHTTP("http://whatever.anydomain/page.html", time.Second, 15)
		assert.Exactly(t, `A response long`, string(content))
		assert.NoError(t, err)
	})

	t.Run("Error Reading body", func(t *testing.T) {
		haveErr := errors.NewAlreadyClosedf("Brain already closed")
		esitag.TestClient = &http.Client{
			Transport: esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", haveErr),
		}

		content, err := esitag.FetchHTTP("http://whatever.anydomain/page.html", time.Second, 15)
		assert.Empty(t, content)
		assert.Contains(t, err.Error(), `Brain already closed`)
	})

	t.Run("Timeout", func(t *testing.T) {
		t.Skip("Maybe todo")
	})
}
