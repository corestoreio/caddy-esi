package bufpool_test

import (
	"testing"

	"github.com/corestoreio/csfw/util/bufferpool"
	"github.com/stretchr/testify/assert"
)

func TestBufferPoolSize(t *testing.T) {
	p := bufferpool.New(4096)
	assert.Exactly(t, 4096, p.Get().Cap())
	assert.Exactly(t, 0, p.Get().Len())
}
