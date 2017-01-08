// Copyright 2016-2017, Cyrill @ Schumacher.fm and the CaddyESI Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package esitag

import (
	"bufio"
	"bytes"
	"io"
	"sync"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
)

// MaxSizeESITag maximum size of an ESI tag. For now this value has been
// returned from a dice roll.
const MaxSizeESITag = 4096

// Parse parses a stream of data to extract ESI Tags. Malformed ESI tags won't
// trigger any errors, instead the parser skips them.
func Parse(r io.Reader) (Entities, error) {
	ret := make(Entities, 0, 5) // avg 5 tags per parse ...

	sc := bufio.NewScanner(r)
	buf := bufpool.Get()
	defer bufpool.Put(buf)
	sc.Buffer(buf.Bytes(), cap(buf.Bytes())+2)

	fdr := finderPoolGet()
	defer finderPoolPut(fdr)
	sc.Split(fdr.split)

	var tagIndex int
	for sc.Scan() {
		if sc.Err() != nil {
			return nil, errors.Wrap(sc.Err(), "[esitag] Parse scan failed")
		}

		ret = append(ret, &Entity{
			Log:    log.BlackHole{},
			RawTag: sc.Bytes(),
			DataTag: DataTag{
				Start: fdr.begin,
				End:   fdr.end,
			},
		})
		tagIndex++
	}

	if err := ret.ParseRaw(); err != nil {
		return nil, errors.Wrap(err, "[esitag] Slice.ParseRaw")
	}
	return ret, nil
}

var finderPool = sync.Pool{
	New: func() interface{} {
		return newFinder(MaxSizeESITag)
	},
}

func finderPoolGet() *finder {
	return finderPool.Get().(*finder)
}

func finderPoolPut(tf *finder) {
	tf.buf.Reset()
	tf.tagState = stateStart
	tf.n = 0
	finderPool.Put(tf)
}

type tagState int

const (
	stateStart  tagState = iota + 1
	stateTag             // read <
	stateTagE            // read <e
	stateTagES           // read <es
	stateTagESI          // read <esi
	//stateTagESIc          // read <esi:
	stateData  // now reading stuff behind :
	stateSlash // found / which might be start of />
	stateFound // found /> as end of esi tag
)

// finder represents a state machine
type finder struct {
	tagState
	n          int
	begin, end int
	buf        *bytes.Buffer
}

func newFinder(bufCap int) *finder {
	return &finder{
		tagState: stateStart,
		buf:      bytes.NewBuffer(make([]byte, 0, bufCap)), // for now max size of one esi tag
	}
}

// split used in bufio.Scanner and matches the signature of bufio.SplitFunc. the
// variable names for the returned values are for documentation purposes only.
func (e *finder) split(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		ok, err := e.scan(b)
		if err != nil {
			return 0, nil, errors.Wrap(err, "[esitag] finder split scan failed")
		}
		if ok {
			return i, e.data(), nil
		}
	}
	return len(data), nil, nil
}

// scan scans the next byte in the input stream and returns whether a <esi: ...
// /> tag was found in which case a call to data() reveals the what the ...
// matched.
func (e *finder) scan(b byte) (bool, error) {
	switch e.tagState {
	case stateStart, stateFound:
		if b == '<' {
			e.tagState = stateTag
			e.begin = e.n
		}
	case stateTag:
		e.tagState = stateStart
		if b == 'e' {
			e.tagState = stateTagE
		}
	case stateTagE:
		e.tagState = stateStart
		if b == 's' {
			e.tagState = stateTagES
		}
	case stateTagES:
		e.tagState = stateStart
		if b == 'i' {
			e.tagState = stateTagESI
		}
	case stateTagESI:
		e.tagState = stateStart
		if b == ':' {
			e.tagState = stateData
			e.buf.Reset()
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
			return true, nil
		}
		e.buf.WriteByte(b)
		e.tagState = stateData
	default:
		return false, errors.NewNotImplementedf("[esitag] Parser detected an unknown state in machine: %d with Byte: %q", e.tagState, rune(b))
	}
	e.n++
	return false, nil
}

// Data returns the content of the esi tag <esi:(content)>/> as well
// as the byte position of the begin and end of the whole tag.
// The returned slice is safe for further usage.
func (e *finder) data() []byte {
	if e.tagState != stateFound {
		return nil
	}
	data := e.buf.Bytes()

	ret := make([]byte, len(data)-1)
	copy(ret, data[:len(data)-1]) // trim last /
	return ret
}
