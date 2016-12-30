package backend

import (
	"time"

	"github.com/corestoreio/errors"
)

// FetchRPC TODO fetch from a RPC service.
func FetchRPC(url string, timeout time.Duration, maxBodySize uint64) ([]byte, error) {
	return nil, errors.NewNotSupportedf("[esibackend] Not yet supported for URL %q", url)
}
