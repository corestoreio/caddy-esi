package backend_test

import (
	"testing"
	"time"

	"fmt"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

var _ fmt.Stringer = (*backend.Resource)(nil)

func TestNewResource(t *testing.T) {
	t.Run("URL", func(t *testing.T) {
		r, err := backend.NewResource(0, "http://cart.service")
		if err != nil {
			t.Fatalf("%+v", err)
		}
		assert.Exactly(t, `http://cart.service`, r.String())
	})

	t.Run("URL is an alias", func(t *testing.T) {
		r, err := backend.NewResource(0, "awsRedisCartService")
		if err != nil {
			t.Fatalf("%+v", err)
		}
		assert.Exactly(t, `awsRedisCartService`, r.String())
	})

	t.Run("URL scheme not found", func(t *testing.T) {
		r, err := backend.NewResource(0, "ftp://cart.service")
		assert.Nil(t, r)
		assert.True(t, errors.IsNotSupported(err), "%+v", err)
	})

	t.Run("URL Template", func(t *testing.T) {
		r, err := backend.NewResource(0, "http://cart.service?product={{ .r.Header.Get }}")
		if err != nil {
			t.Fatalf("%+v", err)
		}
		assert.Exactly(t, "http://cart.service?product={{ .r.Header.Get }} Template: resource_tpl", r.String())
	})

	t.Run("URL Template throws fatal error", func(t *testing.T) {
		r, err := backend.NewResource(0, "http://cart.service?product={{ r.Header.Get }}")
		assert.Nil(t, r)
		assert.True(t, errors.IsFatal(err), "%+v", err)
	})
}

func TestResource_CircuitBreaker(t *testing.T) {
	t.Parallel()

	r, err := backend.NewResource(1, "http://to/a/location")
	if err != nil {
		t.Fatalf("%+v", err)
	}
	state, lastFailure := r.CBState()
	assert.Exactly(t, backend.CBStateClosed, state, "CBStateClosed")
	assert.Exactly(t, time.Unix(1, 0), lastFailure, "lastFailure")

	assert.Exactly(t, uint64(0), r.CBFailures(), "CBFailures()")
	fail := r.CBRecordFailure()
	assert.True(t, fail > 0, "Timestamp greater 0")

	fail = r.CBRecordFailure()
	assert.True(t, fail > 0, "Timestamp greater 0")

	state, lastFailure = r.CBState()
	assert.Exactly(t, backend.CBStateClosed, state, "CBStateClosed")
	assert.True(t, lastFailure.UnixNano() > fail, "lastFailure greater than recorded failure")

	assert.Exactly(t, uint64(2), r.CBFailures(), "CBFailures()")

	var i uint64
	for ; i < backend.CBMaxFailures; i++ {
		r.CBRecordFailure()
	}
	assert.Exactly(t, 14, int(r.CBFailures()), "CBFailures()")

	state, lastFailure = r.CBState()
	assert.Exactly(t, backend.CBStateOpen, state, "CBStateOpen")
	assert.True(t, lastFailure.UnixNano() > fail, "lastFailure greater than recorded failure")

}
