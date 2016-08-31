package esi

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Conditioner interface { // no, not the shampoo ;-)
	OK(r *http.Request) bool
}

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

type ESITags []*ESITag

const maxSizeESITag = 4096

// ParseESITags parses a stream of HTML data to look for ESI Tags. If found it
// returns all tags.
func ParseESITags(r io.Reader) (ret ESITags, _ error) {
	ret = make(ESITags, 0, 5) // avg 5 tags per parse ...

	sc := bufio.NewScanner(r)

	ef := newTagFinder(maxSizeESITag)
	sc.Split(ef.split)

	var tagIndex int
	for sc.Scan() {
		if sc.Err() != nil {
			return nil, sc.Err()
		}
		tag := sc.Bytes()

		ret = append(ret, &ESITag{
			RawTag:   make([]byte, len(tag)),
			TagStart: ef.begin,
			TagEnd:   ef.end,
		})
		copy(ret[tagIndex].RawTag, tag)
		tagIndex++
	}
	return ret, nil
}

type tagState int

const (
	stateStart   tagState = iota
	stateTag              // read <
	stateTagE             // read <e
	stateTagES            // read <es
	stateTagESI           // read <esi
	stateTagESIc          // read <esi:
	stateData             // now reading stuff behind :
	stateSlash            // found / which might be start of />
	stateFound            // found /> as end of esi tag
)

// tagFinder represents a state machine
type tagFinder struct {
	tagState
	n          int
	begin, end int
	buf        *bytes.Buffer
}

func newTagFinder(bufCap int) *tagFinder {
	return &tagFinder{
		tagState: stateStart,
		buf:      bytes.NewBuffer(make([]byte, 0, bufCap)), // for now max size of one esi tag
	}
}

func (e *tagFinder) split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		ok, err := e.scan(b)
		if err != nil {
			return 0, nil, err
		}
		if ok {
			return i, e.data(), nil
		}
	}
	return len(data), nil, nil
}

// Scan scans the next byte in the input stream and returns
// whether a <esi: ... /> tag was found in which case a call to
// Data reveals the what the ... matched.
func (e *tagFinder) scan(b byte) (bool, error) {
	switch e.tagState {
	case stateStart, stateFound:
		if b == '<' {
			e.tagState = stateTag
			e.begin = e.n
		}
	case stateTag:
		if b == 'e' {
			e.tagState = stateTagE
		} else {
			e.tagState = stateStart
		}
	case stateTagE:
		if b == 's' {
			e.tagState = stateTagES
		} else {
			e.tagState = stateStart
		}
	case stateTagES:
		if b == 'i' {
			e.tagState = stateTagESI
		} else {
			e.tagState = stateStart
		}
	case stateTagESI:
		if b == ':' {
			e.tagState = stateData
			e.buf.Reset()
		} else {
			e.tagState = stateStart
		}
	case stateData:
		e.buf.WriteByte(b)
		if b == '/' {
			e.tagState = stateSlash
		}
	case stateSlash:
		if b == '>' {
			e.tagState = stateFound
			e.end = e.n + 1 // to also exclude the >.
			e.n++
			return true, nil
		}
		e.buf.WriteByte(b)
		e.tagState = stateData
	default:
		return false, fmt.Errorf("[caddyesi] Unknown state in machine: %d with Byte: %q", e.tagState, rune(b))
	}
	e.n++
	return false, nil
}

// Data returns the content of the esi tag <esi:(content)>/> as well
// as the byte position of the begin and end of the whole tag.
func (e *tagFinder) data() []byte {
	if e.tagState != stateFound {
		return nil
	}
	data := e.buf.Bytes()
	// trim last /
	return data[:len(data)-1] //, e.begin, e.end
}
