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
	esiTagStart    = []byte(`<esi:include`)
	esiTagStartLen = len(esiTagStart)
	esiTagEnd      = []byte(`/>`)
)

// ParseESITags parses a stream of HTML data to look for ESI Tags
func ParseESITags(r io.Reader) (ret ESITags,_ error) {
	const innerBufSize = 64
	const outerBufSize = innerBufSize * 4

	br := bufio.NewReader(r)

	buffer := make([]byte, innerBufSize)
	var globalPos int
	var startPos = -1
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

		if startPos < 0 {
			startPos = bytes.Index(data, esiTagStart)
			if startPos < 0 {
				// look ahead without advancing the reader
				peek, err := br.Peek(esiTagStartLen)
				if err == nil {
					data = append(data, peek...)
					startPos = bytes.Index(data, esiTagStart)
					if startPos > -1 { // yay found via look ahead! so advance the reader
						if _,err := br.Discard(esiTagStartLen); err != nil {
							panic(err) // todo
						}
					}
				}
			}

			fmt.Printf("Start: globalPos %03d startPos %04d DATA: %q\n", globalPos, startPos, data)

			if startPos < 0 {
				clearBuffer(buffer)
				continue
			}
		}

		endPos := bytes.Index(data[startPos:], esiTagEnd)
		// todo look adhead

		fmt.Printf("End: globalPos %03d startPos %04d endPos %04d DATA: %q\n", globalPos, startPos,endPos, data)


		if endPos < 0 {
			clearBuffer(buffer)
			continue
		}



		println("2: ", string(data), startPos, endPos)
	}

	return nil, nil
}

func clearBuffer(buffer []byte) {
	buffer = buffer[:cap(buffer)]
	n := len(buffer)
	for i := 0; i < n; i++ {
		buffer[i] = 0
	}
}
