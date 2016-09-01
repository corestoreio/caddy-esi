package esi

import (
	"fmt"
	"net/http"
	"time"
)

type Conditioner interface { // no, not the shampoo ;-)
	OK(r *http.Request) bool
}

// ESITag represents a single ESI tag
type ESITag struct {
	RawTag         []byte
	TagStart       int // byte slice index position in the whole slice
	TagEnd         int // byte slice index position in the whole slice
	Sources        []fmt.Stringer
	Key            []byte
	TTL            time.Duration
	Timeout        time.Duration
	OnError        string
	ForwardHeaders []string
	ReturnHeaders  []string
	Conditioner
}

// ESITags represents a list of ESI tags
type ESITags []*ESITag
