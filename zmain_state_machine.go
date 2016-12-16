// +build ignore

package main

import (
	"bytes"
	"fmt"
)

type state int

const (
	stateStart   state = iota
	stateTag           // read <
	stateTagE          // read <e
	stateTagES         // read <es
	stateTagESI        // read <esi
	stateTagESIc       // read <esi:
	stateData          // now reading stuff behind :
	stateSlash         // found / which might be start of />
	stateFound         // found /> as end of esi tag
)

type ESIFinder struct {
	state      state
	n          int
	begin, end int
	buf        *bytes.Buffer
}

func NewESIFinder() *ESIFinder {
	return &ESIFinder{
		state: stateStart,
		buf:   bytes.NewBuffer(make([]byte, 0, 255)),
	}
}

// Scan scans the next byte in the input stream and returns
// wheter a <esi: ... /> tag was found in which case a call to
// Data reveals the what the ... matched.
func (e *ESIFinder) Scan(b byte) bool {
	switch e.state {
	case stateStart, stateFound:
		if b == '<' {
			e.state = stateTag
			e.begin = e.n
		}
	case stateTag:
		if b == 'e' {
			e.state = stateTagE
		} else {
			e.state = stateStart
		}
	case stateTagE:
		if b == 's' {
			e.state = stateTagES
		} else {
			e.state = stateStart
		}
	case stateTagES:
		if b == 'i' {
			e.state = stateTagESI
		} else {
			e.state = stateStart
		}
	case stateTagESI:
		if b == ':' {
			e.state = stateData
			e.buf.Reset()
		} else {
			e.state = stateStart
		}
	case stateData:
		e.buf.WriteByte(b)
		if b == '/' {
			e.state = stateSlash
		}
	case stateSlash:
		if b == '>' {
			e.state = stateFound
			e.end = e.n
			e.n++
			return true
		}
		e.buf.WriteByte(b)
		e.state = stateData
	default:
		panic("ooops")
	}

	e.n++
	return false
}

// Data returns the content of the esi tag <esi:(content)>/> as well
// as the byte position of the begin and end of the whole tag.
func (e *ESIFinder) Data() ([]byte, int, int) {
	if e.state != stateFound {
		return nil, 0, 0
	}
	data := e.buf.Bytes()
	// trim /
	return data[:len(data)-1], e.begin, e.end
}

func main() {
	data := []byte("Hello<html><emp>a</emph><esi:include X/>blob")
	esf := NewESIFinder()

	for i, b := range data {
		if found := esf.Scan(b); found {
			data, begin, end := esf.Data()
			fmt.Printf("%d: [%d,%d] = %s\n", i, begin, end, data)
		}
	}

}
