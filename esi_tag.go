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

var (
	esiTagStart = []byte(`<esi:include`)
	esiTagEnd   = []byte(`/>`)
)

// ParseESITags parses a stream of HTML data to look for ESI Tags
func ParseESITags(r io.Reader) (ret ESITags, _ error) {
	const innerBufSize = 2048 // min size len(esiTagStart)

	br := bufio.NewReader(r)

	buffer := make([]byte, innerBufSize)
	var globalPos int
	var startPos = -1

	var tagIndex = -1
	var totalTagEnds int
	for {
		n, err := br.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("[caddyesi] Failed to read from buffer: %s", err)
		}
		globalPos += n
		data := buffer[:n]

		//bufferDirectHit:
		// special case direct hit of start and end tag within the buffer slice
		//fmt.Printf("tagIndeX %d %q\n\n", tagIndex, data)

		for tsp, tep := bytes.Index(data, esiTagStart), bytes.Index(data, esiTagEnd); tsp > -1 && tsp < tep; {

			tep += len(esiTagEnd)
			ret = append(ret, &ESITag{
				RawTag:   make([]byte, tep-tsp),
				TagStart: globalPos,
			})
			tagIndex++
			copy(ret[tagIndex].RawTag, data[tsp:tep])
			totalTagEnds++

			//fmt.Printf("tagIndex %d %q => tsp %d tep %d\n", tagIndex, data[tsp:tep],tsp, tep)

			data = data[tep:]
			// recalculate positions in the new slice
			tsp, tep = bytes.Index(data, esiTagStart), bytes.Index(data, esiTagEnd)
		}

		//fmt.Printf("tagIndeX %d %q\n\n", tagIndex, data)

		// start more in-depth search with lookahead into the next coming buffer

		if startPos < 0 {
			// find start position and do a look ahead. if look ahead, then
			// return the new data slice and forward bufio.Reader
			startPos, data = getPosition(br, data, esiTagStart)

			if startPos < 0 {
				clearBuffer(buffer)
				continue
			}

			//fmt.Printf("Start: globalPos %03d startPos %04d DATA: %q\n", globalPos, startPos, data)

			ret = append(ret, &ESITag{
				RawTag:   make([]byte, 0, 256),
				TagStart: globalPos,
			})
			tagIndex++
			data = data[startPos:] // discard any data before tag start because it might contain "/>"
			globalPos += len(esiTagStart)
		}

		endPosFound := false
		if startPos > -1 { // we know we have found a starting tag
			var ep int
			// we do not know if the end tag already is in the data, even with a look ahead.
			ep, data = getPosition(br, data, esiTagEnd)
			//dataLen := len(data)

			//fmt.Printf("startPos %d | ep %d | newdata: %q\n\n", startPos, ep, data)

			if ep > -1 {
				//ret[tagIndex].RawTag = append(ret[tagIndex].RawTag, data...)
				//ret[tagIndex].RawTag = append(ret[tagIndex].RawTag, data[len(esiTagEnd):]...)
				endPosFound = true
			}

			// as long as we don't have found the end tag, append the data to the RawTag
			ret[tagIndex].RawTag = append(ret[tagIndex].RawTag, data...)
		}

		if !endPosFound {
			clearBuffer(buffer)
			continue
		}
		if startPos > -1 && endPosFound {
			cutOff := bytes.Index(ret[tagIndex].RawTag, esiTagEnd) + len(esiTagEnd)
			//fmt.Printf("End: globalPos %03d startPos %04d endPos %04d DATA: %q\n\n", globalPos, startPos, cutOff, ret[tagIndex].RawTag)
			// trim the RawTag buffer until the EndTag
			ret[tagIndex].RawTag = ret[tagIndex].RawTag[:cutOff]
			ret[tagIndex].TagEnd = globalPos
			startPos = -1
			endPosFound = false
			clearBuffer(buffer)
			totalTagEnds++
		}
	}

	if have, want := totalTagEnds, len(ret); have != want {
		// human error message, to make clear where the bug is.
		buf := new(bytes.Buffer)
		for _, t := range ret {
			fmt.Fprintf(buf, "%q", t.RawTag)
			buf.WriteRune('\n')
		}
		return nil, fmt.Errorf("[caddyesi] Opening close tag mismatch!\n%s", buf)
	}
	return ret, nil
}

// getPosition searches a sep within data
func getPosition(br *bufio.Reader, data, sep []byte) (startPos int, _ []byte) {
	sepLen := len(data)
	startPos = bytes.Index(data, sep)
	if startPos < 0 {
		// look ahead without advancing the reader
		peek, _ := br.Peek(sepLen)

		if len(peek) > 0 {
			//fmt.Printf("PEEK: sep: %q data %q\n\npeek: %q\n\n\n", sep,data, peek)
			dataOrg := data
			data = append(data, peek...) // merge because part of the sep can be in "data" and the other part in "peek"
			startPos = bytes.Index(data, sep)
			if startPos > -1 { // yay found via look ahead! so advance the reader
				if sepLen > len(peek) {
					sepLen = len(peek)
				}
				if _, err := br.Discard(sepLen); err != nil {
					panic(err) // todo remove panic
				}
			} else {
				// not found sep, so reset to previous
				data = dataOrg
			}
		}
	}
	return startPos, data
}

func clearBuffer(buffer []byte) {
	buffer = buffer[:cap(buffer)]
	n := len(buffer)
	for i := 0; i < n; i++ {
		buffer[i] = 0
	}
}
