package backend_test

import (
	"testing"
	"time"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/stretchr/testify/assert"
)

func TestResource_CircuitBreaker(t *testing.T) {
	t.Parallel()

	r := backend.NewResource(1, "anyurl://to/a/location")
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
