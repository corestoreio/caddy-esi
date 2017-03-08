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
	// Data from the micro service gathered in a goroutine.
	Data  []byte
	Start int // Start position in the stream
	End   int // End position in the stream. Never smaller than Start.
}

// String prints human readable the data tag for debugging purposes.
func (dt DataTag) String() string {
	return fmt.Sprintf("Start:%06d End:%06d Tag:%q", dt.Start, dt.End, dt.Data)
}

// DataTags a list of tags with their position within a page and the content
type DataTags struct {
	Slice []DataTag

	//mu sync.RWMutex // TODO maybe remove the lock but first check for races
	// streamPos: first call to InjectContent and the streamPos equals 0
	// because we're at the beginning of the stream. the next call to
	// InjectContent and streamPos gets incremented by the length of the
	// previous data in the Reader. DataTags.InjectContent does not know it gets
	// called multiple times and hence it can inject the Tag tag data in
	// subsequent calls. So streamPos protects recurring replacements.
	posPrev int
	posCur  int
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
	const writeErr = "[esitag] DataTags.InjectContent: Failed to write middleware data to writer: %q for tag index %d with start position %d and end position %d"

	nRead = len(data)
	dts.posCur += len(data)

	tagWritten := false
	for di, dt := range dts.Slice {

		esiStartPos := len(data) - (dt.End - dt.Start)

		switch {
		case dt.Start >= dts.posPrev && dt.End <= dts.posCur:
			// tags replacement fits directly into the start and end position.

			if len(data) > dt.Start {
				esiStartPos = dt.Start
			}

			wn, errW := w.Write(data[:esiStartPos])
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
			}

			wn, errW = w.Write(dt.Data)
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
			}

			esiEndPos := esiStartPos + (dt.End - dt.Start)
			if len(data) > dt.End {
				esiEndPos = dt.End
			}

			wn, errW = w.Write(data[esiEndPos:])
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
			}
			tagWritten = true

		case dt.Start >= dts.posPrev && dt.Start <= dts.posCur:
			// start position is between previous and current position, which
			// means that the tag begins here somewhere to start.

			wn, errW := w.Write(data[:dt.Start-dts.posPrev])
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
			}

			wn, errW = w.Write(dt.Data)
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
			}
			tagWritten = true

		case dt.Start < dts.posCur && dt.Start < dts.posPrev && dt.End > dts.posPrev && dt.End > dts.posCur:
			// we're in the middle of writing the tag but we must discard the
			// ESI tag itself. Hence we say that the tag has been written.
			tagWritten = true

		case dt.End >= dts.posPrev && dt.End <= dts.posCur:
			// reached end of ESI tag and write the last chunk
			wn, errW := w.Write(data[len(data)-(dts.posCur-dt.End):])
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
			}
			tagWritten = true
		}
	}

	if !tagWritten {
		n, err := w.Write(data)
		nWritten += n
		if err != nil {
			return nRead, nWritten, errors.NewWriteFailedf("[esitag] InjectContent failed to copy remaining data to w: %s", err)
		}
	}

	dts.posPrev += len(data)

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
