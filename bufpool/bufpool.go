package bufpool

import (
	"bytes"
	"sync"
)

const bufferMaxSize = 1 << 18 // 256KiB

var bufferPool = New(8 * 1024) // estimated *cough* average size

// Get returns a buffer from the pool.
func Get() *bytes.Buffer {
	return bufferPool.Get()
}

// Put returns a buffer to the pool.
// The buffer is reset before it is put back into circulation.
func Put(buf *bytes.Buffer) {
	// @see https://go-review.googlesource.com/c/go/+/136116/4/src/fmt/print.go
	// Proper usage of a sync.Pool requires each entry to have approximately
	// the same memory cost. To obtain this property when the stored type
	// contains a variably-sized buffer, we add a hard limit on the maximum buffer
	// to place back in the pool.
	//
	// See https://golang.org/issue/23199
	if buf.Cap() > bufferMaxSize {
		return
	}
	bufferPool.Put(buf)
}

// Tank implements a sync.Pool for bytes.Buffer
type Tank struct {
	p *sync.Pool
}

// Get returns type safe a buffer
func (t Tank) Get() *bytes.Buffer {
	return t.p.Get().(*bytes.Buffer)
}

// Put empties the buffer and returns it back to the pool.
//
//		bp := New(320)
//		buf := bp.Get()
//		defer bp.Put(buf)
//		// your code
//		return buf.String()
//
// If you use Bytes() function to return bytes make sure you copy the data
// away otherwise your returned byte slice will be empty.
// For using String() no copying is required.
func (t Tank) Put(buf *bytes.Buffer) {
	// @see https://go-review.googlesource.com/c/go/+/136116/4/src/fmt/print.go
	// Proper usage of a sync.Pool requires each entry to have approximately
	// the same memory cost. To obtain this property when the stored type
	// contains a variably-sized buffer, we add a hard limit on the maximum buffer
	// to place back in the pool.
	//
	// See https://golang.org/issue/23199
	if buf.Cap() > bufferMaxSize {
		return
	}
	buf.Reset()
	t.p.Put(buf)
}

// New instantiates a new bytes.Buffer pool with a custom
// pre-allocated buffer size.
func New(size int) Tank {
	return Tank{
		p: &sync.Pool{
			New: func() interface{} {
				b := bytes.NewBuffer(make([]byte, size))
				b.Reset()
				return b
			},
		},
	}
}
