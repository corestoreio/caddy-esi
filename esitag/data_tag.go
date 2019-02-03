// Copyright 2015-present, Cyrill @ Schumacher.fm and the CoreStore contributors
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
	"bytes"
	"fmt"
	"io"

	"github.com/corestoreio/errors"
)

// DataTag identifies an Tag tag by its start and end position in the HTML byte
// stream for replacing. If the HTML changes there needs to be a refresh call to
// re-parse the HTML.
type DataTag struct {
	Data  []byte // Data from the micro service gathered in a goroutine. Can be nil.
	Start int    // Start position in the stream
	End   int    // End position in the stream. Never smaller than Start.
}

// String prints human readable the data tag for debugging purposes.
func (dt DataTag) String() string {
	return fmt.Sprintf("Start:%06d End:%06d Tag:%q", dt.Start, dt.End, dt.Data)
}

// DataTags a list of tags with their position within a page and the content
type DataTags struct {
	Slice []DataTag

	// streamPrev: first call to InjectContent and the streamPrev equals 0
	// because we're at the beginning of the stream. The next call to
	// InjectContent and streamPrev gets incremented by the length of the
	// previous data. DataTags.InjectContent does not know it gets called
	// multiple times and hence it can inject the DataTag data in subsequent
	// calls. So streamPrev protects repeated replacements.
	streamPrev int
	// streamCur see streamPrev but streamCur contains the current position aka.
	// data length.
	streamCur int
	// writeStates contains for each Slice index the state of the replacement. 0
	// = not yet started, 1 = replacement in progress and waiting for more
	// incoming chunks, 2 = replacement done.
	writeStates []uint8 // 0 nada, 1 in progress, 2 done
}

// NewDataTagsCapped creates a new object with a DataTag slice and its maximum
// capacity.
func NewDataTagsCapped(cap int) *DataTags {
	return &DataTags{
		Slice: make([]DataTag, 0, cap),
	}
}

// fullNextTag looks ahead if the next tag is fully contained in the current
// data slice. Returns the relative start position of the next tag. Returns also
// true if we reach the last tag in the Slice.
func (dts *DataTags) fullNextTag(nextIdx, dataLen int) (relPosStart int, hasNext, isLast bool) {
	if nextIdx > dts.Len() {
		return 0, false, false
	}
	if nextIdx == dts.Len() { // reached end of slice
		return dataLen, true, true
	}

	dt := dts.Slice[nextIdx]
	relPosStart = dt.Start - dts.streamPrev
	relPosEnd := dataLen - (dts.streamCur - dt.End)
	hasFullTagInData := dt.Start >= dts.streamPrev && dt.End <= dts.streamCur && relPosStart > 0 && relPosEnd > 0 && relPosStart < dataLen && relPosEnd <= dataLen
	return relPosStart, hasFullTagInData, false
}

// ResetStates exported for Benchmarks. Resets the internal state machine to
// re-run the injector without instantiating a new object.
func (dts *DataTags) ResetStates() {
	for k := range dts.writeStates {
		dts.writeStates[k] = 0
	}
	dts.streamCur = 0
	dts.streamPrev = 0
}

// InjectContent inspects data argument and uses the data field in a DataTag
// type to injected the backend data at the current position in the data
// argument and then writes the output to w. DataTags must be a sorted slice.
// Usually this function receives the data from Entities.QueryResources(). This
// function can be called multiple times. It tracks the stream position and
// inserts the ESI tag once the correct position has been reached. This function
// cannot yet be used in parallel.
func (dts *DataTags) InjectContent(data []byte, w io.Writer) (nWritten int, _ error) {
	if dts.writeStates == nil {
		dts.writeStates = make([]uint8, len(dts.Slice))
	}

	const writeErr = "[esitag] DataTags.InjectContent: Failed to write data into w for TagIndex[%d] StartPos[%d] EndPos[%d]"
	const (
		writeStateWaiting uint8 = iota
		writeStateProgress
		writeStateDone
	)

	//fmt.Printf("INPUT: %q\n", data)

	dataLen := len(data)

	if dts.Len() == 0 || dataLen == 0 {
		wn, err := w.Write(data)
		nWritten += wn
		return nWritten, errors.WriteFailed.New(err, "[esitag] Failed to write")
	}

	dts.streamCur += dataLen

	tagWritten := false
	prevRelPosEnd := 0
	for di, dt := range dts.Slice {
		// - InjectContent can be called n-times with an unknown amount of streamed data.
		// - one DataTag can only be inserted once
		//   - DataTag can occur completely in `data`
		//   - DataTag cannot be found in `data`
		//   - DataTag can have a start position but no End position
		//   - DataTag can wait until the data comes in with the end position
		//   - DataTags can occur 1+n times in the data; complex case

		relPosStart := dt.Start - dts.streamPrev
		relPosEnd := dataLen - (dts.streamCur - dt.End)

		hasPosStartInData := relPosStart < dataLen && relPosEnd > dataLen && dt.Start < dts.streamCur
		hasPosEndInData := relPosStart < 0 && relPosEnd > 0 && relPosEnd < dataLen && dt.End <= dts.streamCur
		hasFullTagInData := dt.Start >= dts.streamPrev && dt.End <= dts.streamCur && relPosStart > 0 && relPosEnd > 0 && relPosStart < dataLen && relPosEnd <= dataLen

		//fmt.Printf("TagID[%d]: Data[%03d] dt.Start[%03d] dt.End[%03d] dts.streamPrev[%03d] dts.streamCur[%03d] relPosStart[%03d] relPosEnd[%03d] hasPosStartInData[%t] hasPosEndInData[%t] hasFullTagInData[%t]\n",
		//	di, dataLen, dt.Start, dt.End, dts.streamPrev, dts.streamCur, relPosStart, relPosEnd,
		//	hasPosStartInData, hasPosEndInData, hasFullTagInData)

		switch dts.writeStates[di] {
		case writeStateDone:
			// do nothing
		case writeStateProgress:
			if hasPosEndInData {
				wn, err := w.Write(data[relPosEnd:])
				if err != nil {
					return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
				}
				nWritten += wn
				dts.writeStates[di] = writeStateDone
			}
			tagWritten = true
		case writeStateWaiting:
			switch {
			case hasFullTagInData:
				// data can contain:
				// - one esi tag: write before, write tag, write after, hasNext is false!
				// - two esi tags where we need to write the middle part between the ESI tags
				// - n ESI tags.

				// look ahead to see if the next DataTag is fully contained in the current data
				// byte slice.
				if nextStartPos, hasNext, isLast := dts.fullNextTag(di+1, dataLen); hasNext {
					//fmt.Printf("TagID[%d] nextStartPos[%d] prevRelPosEnd[%d]\n", di, nextStartPos, prevRelPosEnd)
					//fmt.Printf("TagID[%d] before: %q\n", di, data[prevRelPosEnd:relPosStart])
					//fmt.Printf("TagID[%d] after: %q\n", di, data[relPosEnd:nextStartPos])
					wn, err := w.Write(data[prevRelPosEnd:relPosStart])
					if err != nil {
						return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
					}
					nWritten += wn

					wn, err = w.Write(dt.Data)
					if err != nil {
						return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
					}
					nWritten += wn

					if isLast {
						wn, err := w.Write(data[relPosEnd:nextStartPos])
						if err != nil {
							return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
						}
						nWritten += wn
					}
				} else {
					// We only have one full tag in the current data slice.
					// write before tag, write DataTag itself, then write the remaining chunks
					wn, err := w.Write(data[:relPosStart])
					if err != nil {
						return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
					}
					nWritten += wn

					wn, err = w.Write(dt.Data)
					if err != nil {
						return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
					}
					nWritten += wn

					wn, err = w.Write(data[relPosEnd:])
					if err != nil {
						return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
					}
					nWritten += wn
				}
				dts.writeStates[di] = writeStateDone
				prevRelPosEnd = relPosEnd
				tagWritten = true

			case hasPosStartInData:
				wn, err := w.Write(data[:relPosStart])
				if err != nil {
					return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
				}
				nWritten += wn

				wn, err = w.Write(dt.Data)
				if err != nil {
					return nWritten, errors.WriteFailed.New(err, writeErr, di, dt.Start, dt.End)
				}
				nWritten += wn

				dts.writeStates[di] = writeStateProgress
				tagWritten = true
			}
		}
	}

	if !tagWritten {
		n, err := w.Write(data)
		nWritten += n
		if err != nil {
			return nWritten, errors.WriteFailed.New(err, "[esitag] InjectContent failed to copy remaining data to w")
		}
	}

	dts.streamPrev += dataLen

	return nWritten, nil
}

// DataLen returns the total length of all data fields in bytes.
func (dts *DataTags) DataLen() (l int) {
	for _, dt := range dts.Slice {
		// subtract the length of the raw Tag tag (end-start) from the data
		// length to get the correct length of the inserted data for the
		// Content-Length header. End can never be smaller than Start. The sum
		// can be negative, means the returned data from the backend resource is
		// shorter than the ESI tag itself.
		l += len(dt.Data) - (dt.End - dt.Start)
	}
	return
}

func (dts *DataTags) Len() int           { return len(dts.Slice) }
func (dts *DataTags) Swap(i, j int)      { dts.Slice[i], dts.Slice[j] = dts.Slice[j], dts.Slice[i] }
func (dts *DataTags) Less(i, j int) bool { return dts.Slice[i].Start < dts.Slice[j].Start }

// String used for debug output during development
func (dts *DataTags) String() string {
	var buf bytes.Buffer
	for i, t := range dts.Slice {
		fmt.Fprintf(&buf, "IDX(%d/%d): %s\n", i+1, dts.Len(), t)
	}
	return buf.String()
}
