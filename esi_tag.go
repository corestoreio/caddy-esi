package esi

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Conditioner interface { // no, not the shampoo ;-)
	OK(r *http.Request) bool
}

type ESITag struct {
	RawTag         []byte
	Sources        []fmt.Stringer
	Key            []byte
	TTL            time.Duration
	Timeout        time.Duration
	OnError        string
	ForwardHeaders []string
	ReturnHeaders  []string
	Conditioner
}

type ESITags []*ESITag

// For each incoming request we know in advance who many ESI tags there are.
type RequestESITags struct {
	sync.RWMutex
	cache map[uint64]ESITags
}

func NewRequestESITags() RequestESITags {
	return RequestESITags{
		cache: make(map[uint64]ESITags),
	}
}

func NewESITags(r io.Reader) (ESITags, error) {

	return nil, nil
}
