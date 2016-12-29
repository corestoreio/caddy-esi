package backend_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

var _ backend.ResourceRequestFunc = backend.FetchHTTP

func TestFetchHTTP(t *testing.T) {
	// All tests modifying TestClient cannot be run in parallel.

	t.Run("LimitedReader", func(t *testing.T) {
		backend.TestClient = &http.Client{
			Transport: esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", nil),
		}

		content, err := backend.FetchHTTP("http://whatever.anydomain/page.html", time.Second, 15)
		assert.Exactly(t, `A response long`, string(content))
		assert.NoError(t, err)
	})

	t.Run("Error Reading body", func(t *testing.T) {
		haveErr := errors.NewAlreadyClosedf("Brain already closed")
		backend.TestClient = &http.Client{
			Transport: esitesting.NewHTTPTrip(200, "A response longer than 15 bytes", haveErr),
		}

		content, err := backend.FetchHTTP("http://whatever.anydomain/page.html", time.Second, 15)
		assert.Empty(t, content)
		assert.Contains(t, err.Error(), `Brain already closed`)
	})

	t.Run("Timeout", func(t *testing.T) {
		t.Skip("Maybe todo")
	})
}
