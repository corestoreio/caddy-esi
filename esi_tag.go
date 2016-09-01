package esi

import (
	"fmt"
	"net/http"
	"time"
)

type source string

func (u source) String() string {
	return u
}

type Conditioner interface { // no, not the shampoo ;-)
	OK(r *http.Request) bool
}

// ESITag represents a single ESI tag
type ESITag struct {
	RawTag         []byte
	TagStart       int            // start position in the stream
	TagEnd         int            // end position in the stream
	Sources        []fmt.Stringer // creates an URL to fetch data from
	Key            []byte         // use for lookup in key/value storages to fetch data from
	TTL            time.Duration
	Timeout        time.Duration
	OnError        string
	ForwardHeaders []string
	ReturnHeaders  []string
	Conditioner
}

// ParseRaw parses the RawTag field and fills the remaining fields of the
// struct.
func (et *ESITag) ParseRaw() error {
	if len(et.RawTag) == 0 {
		return nil
	}

	// r := bytes.NewReader(et.RawTag)

	return nil
}

// ESITags represents a list of ESI tags
type ESITags []*ESITag
