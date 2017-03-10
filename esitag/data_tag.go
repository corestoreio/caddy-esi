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

//func (dt DataTag) writeFull(relPosStart, relPosEnd int, data []byte, w io.Writer) (n int, _ error) {
//	n2, err := w.Write(data[:relPosStart])
//	if err != nil {
//		return n, errors.NewWriteFailed(err, "[esitag] Write failed. Start %d End %d", dt.Start, dt.End)
//	}
//	n += n2
//	n, err = w.Write(dt.Data)
//	if err != nil {
//		return n, errors.NewWriteFailed(err, "[esitag] Write failed. Start %d End %d", dt.Start, dt.End)
//	}
//	n += n2
//	//n, err = w.Write(data[relPosEnd:])
//	//if err != nil {
//	//	return n, errors.NewWriteFailed(err, "[esitag] Write failed. Start %d End %d", dt.Start, dt.End)
//	//}
//	//n += n2
//	return n, nil
//}

// DataTags a list of tags with their position within a page and the content
type DataTags struct {
	Slice []DataTag

	// streamPrev: first call to InjectContent and the streamPrev equals 0
	// because we're at the beginning of the stream. the next call to
	// InjectContent and streamPrev gets incremented by the length of the
	// previous data. DataTags.InjectContent does not know it gets called
	// multiple times and hence it can inject the Tag tag data in subsequent
	// calls. So streamPrev protects recurring replacements.
	streamPrev int
	// streamCur see streamPrev but streamCur contains the current position aka.
	// data length.
	streamCur int
	// lastMatchedTagStart stores the start position of the last matched tag.
	// Important for multiple calls to InjectContent with data chunks.
	lastMatchedTagStart int
	// lastMatchedTagEnd stores the end position of the last matched tag
	// Important for multiple calls to InjectContent with data chunks.
	lastMatchedTagEnd int
	writeStates       []uint8 // 0 nada, 1 in progress, 2 done
}

// NewDataTagsCapped creates a new object with a DataTag slice and its maximum
// capacity.
func NewDataTagsCapped(cap int) *DataTags {
	return &DataTags{
		Slice: make([]DataTag, 0, cap),
	}
}

// InjectContent inspects data argument and uses the data field in a Tag type to
// injected the backend data at the current position in the data argument and
// then writes the output to w. DataTags must be a sorted slice. Usually this
// function receives the data from Entities.QueryResources(). This function can
// be called multiple times. It tracks the stream position and inserts the ESI
// tag once the correct position has been reached. This function cannot yet be
// used in parallel.
func (dts *DataTags) InjectContent(data []byte, w io.Writer) (nRead, nWritten int, _ error) {
	if dts.writeStates == nil {
		dts.writeStates = make([]uint8, len(dts.Slice))
	}
	const writeErr = "[esitag] DataTags.InjectContent: Failed to writeFull middleware data to writer: %q for tag index %d with start position %d and end position %d"

	const (
		writeStateWaiting uint8 = iota
		writeStateProgress
		writeStateDone
	)

	fmt.Printf("INPUT: %q\n", data)

	nRead = len(data)

	if dts.Len() == 0 || len(data) == 0 {
		wn, err := w.Write(data)
		nWritten += wn
		return nRead, nWritten, errors.NewWriteFailed(err, "[esitag] Failed to writeFull")
	}

	dts.streamCur += len(data)

	tagWritten := false
	//dataStartPos := 0
	prevRelPosEnd := 0
	for di, dt := range dts.Slice {
		// - InjectContent can be called n-times with an unknown amount of streamed data.
		// - one DataTag can only be inserted once
		//   - DataTag can occur completely in `data`
		//   - DataTag cannot be found in `data`
		//   - DataTag can have a start position but no End position
		//   - DataTag can wait until the data comes in with the end position
		//   - DataTags can occur 1+n times in the data

		relPosStart := dt.Start - dts.streamPrev
		relPosEnd := len(data) - (dts.streamCur - dt.End)

		hasPosStartInData := relPosStart < len(data) && relPosEnd > len(data) && dt.Start < dts.streamCur
		hasPosEnd__InData := relPosStart < 0 && relPosEnd > 0 && relPosEnd < len(data) && dt.End <= dts.streamCur
		hasFullTagInData := dt.Start >= dts.streamPrev && dt.End <= dts.streamCur && relPosStart > 0 && relPosEnd > 0 && relPosStart < len(data) && relPosEnd <= len(data)

		fmt.Printf("TagID[%d]: Data[%03d] dt.Start[%03d] dt.End[%03d] dts.streamPrev[%03d] dts.streamCur[%03d] relPosStart[%03d] relPosEnd[%03d] hasPosStartInData[%t] hasPosEnd__InData[%t] hasFullTagInData[%t]\n",
			di, len(data), dt.Start, dt.End, dts.streamPrev, dts.streamCur, relPosStart, relPosEnd,
			hasPosStartInData, hasPosEnd__InData, hasFullTagInData)

		switch dts.writeStates[di] {
		case writeStateDone:
			// do nothing
		case writeStateProgress:
			if hasPosEnd__InData {
				if wn, err := w.Write(data[relPosEnd:]); err != nil {
					return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
				} else {
					nWritten += wn
					dts.writeStates[di] = writeStateDone
				}
			}
			tagWritten = true
		case writeStateWaiting:
			switch {
			case hasFullTagInData:

				switch {
				case di == 0: // first DataTag, we must write all data before the first tag occurs
					if wn, err := w.Write(data[:relPosStart]); err != nil {
						return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
					} else {
						nWritten += wn
					}
					if wn, err := w.Write(dt.Data); err != nil {
						return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
					} else {
						nWritten += wn
					}

				case di == dts.Len()-1: // last DataTag no write all remaining
					if wn, err := w.Write(dt.Data); err != nil {
						return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
					} else {
						nWritten += wn
					}
					if wn, err := w.Write(data[relPosEnd:]); err != nil {
						return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
					} else {
						nWritten += wn
					}
				default: // all other tags in between
					if wn, err := w.Write(data[prevRelPosEnd:relPosStart]); err != nil {
						return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
					} else {
						nWritten += wn
					}
					if wn, err := w.Write(dt.Data); err != nil {
						return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
					} else {
						nWritten += wn
					}

				}
				prevRelPosEnd = relPosEnd
				dts.writeStates[di] = writeStateDone

				//if wn, err := dt.writeFull(relPosStart, relPosEnd, data[prevRelPosEnd:relPosEnd], w); err != nil {
				//	return nRead, nWritten, errors.NewWriteFailed(err, "[esitag] Failed to writeFull")
				//} else {
				//	nWritten += wn

				//	prevRelPosEnd = relPosEnd
				//}
				tagWritten = true
			case hasPosStartInData:
				if wn, err := w.Write(data[:relPosStart]); err != nil {
					return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
				} else {
					nWritten += wn
				}
				if wn, err := w.Write(dt.Data); err != nil {
					return nRead, nWritten, errors.NewWriteFailedf(writeErr, err, di, dt.Start, dt.End)
				} else {
					nWritten += wn
				}
				dts.writeStates[di] = writeStateProgress
				tagWritten = true
			}
		}

		//switch {
		//case dts.writeStates[di] == 0 && dt.Start >= dts.streamPrev && dt.End <= dts.streamCur:
		//	// tags replacement fits directly into the start and end position.
		//
		//	if len(data) > dt.Start {
		//		relStartPos = dt.Start
		//	}
		//
		//	fmt.Printf("DEBUG1/%d: %q\n", di, data[dataStartPos:relStartPos])
		//	wn, errW := w.Write(data[dataStartPos:relStartPos])
		//	nWritten += wn
		//	if errW != nil {
		//		return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
		//	}
		//
		//	wn, errW = w.Write(dt.Data)
		//	nWritten += wn
		//	if errW != nil {
		//		return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
		//	}
		//
		//	esiEndPos := relStartPos + (dt.End - dt.Start)
		//	if len(data) > dt.End {
		//		esiEndPos = dt.End
		//	}
		//	dataStartPos = esiEndPos
		//	dts.lastMatchedTagStart = dt.Start
		//	dts.lastMatchedTagEnd = dt.End
		//	dts.writeStates[di] = 2
		//
		//	//fmt.Printf("DEBUG: Start:%d End:%d DataLen:%d relStartPos:%d esiEndPos:%d dataStartPos:%d\n",
		//	//	dt.Start, dt.End, len(data), relStartPos, esiEndPos, dataStartPos)
		//	//fmt.Printf("DEBUG: Write END: %q\n", data[esiEndPos:nextStartPos])
		//	//
		//	//wn, errW = w.Write(data[esiEndPos:nextStartPos])
		//	//nWritten += wn
		//	//if errW != nil {
		//	//	return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
		//	//}
		//
		//	tagWritten = true
		//
		//case dts.writeStates[di] == 0 && dt.Start >= dts.streamPrev && dt.Start <= dts.streamCur:
		//	// start position is between previous and current position, which
		//	// means that the tag begins here somewhere to start.
		//
		//	fmt.Printf("DEBUG2/%d: Start:%d End:%d DataLen:%d dts.streamPrev:%d dts.streamCur:%d dts.lastMatchedTag(%d/%d)\n",
		//		di, dt.Start, dt.End, len(data), dts.streamPrev, dts.streamCur, dts.lastMatchedTagStart, dts.lastMatchedTagEnd)
		//
		//	var lastPos int
		//	if dts.lastMatchedTagEnd > 0 && dts.lastMatchedTagEnd > dts.streamPrev {
		//		lastPos = dts.lastMatchedTagEnd - dts.streamPrev
		//	}
		//
		//	println("lastPos", lastPos, dt.Start-dts.streamPrev)
		//	fmt.Printf("DEBUG2/%d: %q\n", di, data[lastPos:dt.Start-dts.streamPrev])
		//	wn, errW := w.Write(data[lastPos : dt.Start-dts.streamPrev])
		//	nWritten += wn
		//	if errW != nil {
		//		return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
		//	}
		//
		//	wn, errW = w.Write(dt.Data)
		//	nWritten += wn
		//	if errW != nil {
		//		return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
		//	}
		//	dts.lastMatchedTagStart = dt.Start
		//	dts.lastMatchedTagEnd = dt.End
		//	dts.writeStates[di] = 1
		//	tagWritten = true
		//
		//case dt.Start < dts.streamCur && dt.Start < dts.streamPrev && dt.End > dts.streamPrev && dt.End > dts.streamCur:
		//	// we're in the middle of writing the tag but we must discard the
		//	// ESI tag itself. Hence we say that the tag has been written.
		//	tagWritten = true
		//
		//case dts.writeStates[di] == 1 && dt.End >= dts.streamPrev && dt.End <= dts.streamCur: //  && dt.Start >= dts.streamPrev
		//
		//	fmt.Printf("DEBUG3/%d: Start:%d End:%d DataLen:%d dts.streamPrev:%d dts.streamCur:%d dts.lastMatchedTag(%d/%d)\n",
		//		di, dt.Start, dt.End, len(data), dts.streamPrev, dts.streamCur, dts.lastMatchedTagStart, dts.lastMatchedTagEnd)
		//
		//	// reached end of ESI tag and writeFull the last chunk
		//	fmt.Printf("DEBUG3/%d: %q\n", di, data[len(data)-(dts.streamCur-dt.End):])
		//	if di == 0 || di == dts.Len()-1 {
		//		wn, errW := w.Write(data[len(data)-(dts.streamCur-dt.End):])
		//		nWritten += wn
		//		if errW != nil {
		//			return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
		//		}
		//	}
		//	dts.lastMatchedTagStart = dt.Start
		//	dts.lastMatchedTagEnd = dt.End
		//	tagWritten = true
		//	dts.writeStates[di] = 2
		//}
	}
	//_ = tagWritten
	if !tagWritten {
		//fmt.Printf("DEBUG NOT tagWritten %q\n", data)
		n, err := w.Write(data)
		nWritten += n
		if err != nil {
			return nRead, nWritten, errors.NewWriteFailedf("[esitag] InjectContent failed to copy remaining data to w: %s", err)
		}
	}

	//if dts.Len() > 0 && dts.streamPrev == 0 { // only on first call to InjectContent
	//	lastDT := dts.Slice[dts.Len()-1]
	//	if lastDT.End <= len(data) {
	//		fmt.Printf("DEBUG posPrev0 END %q\n", data)
	//		wn, errW := w.Write(data[lastDT.End:])
	//		nWritten += wn
	//		if errW != nil {
	//			return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, lastDT, lastDT.Start, lastDT.End)
	//		}
	//	}
	//}
	println("\n")

	dts.streamPrev += len(data)

	return nRead, nWritten, nil
}

// DataLen returns the total length of all data fields in bytes.
func (dts *DataTags) DataLen() (l int) {
	for _, dt := range dts.Slice {
		// subtract the length of the raw Tag tag (end-start) from the data
		// length to get the correct length of the inserted data for the
		// Content-Length header. End can never be smaller than Start. The sum
		// can be negative, means the returned data from the backend resource is
		// shorter than the Tag tag itself.
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
