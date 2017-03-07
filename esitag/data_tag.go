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

	"github.com/SchumacherFM/caddyesi/bufpool"
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
	streamPos int
}

// NewDataTags creates a new DataTags slice
func NewDataTags(dts ...DataTag) *DataTags {
	return &DataTags{
		Slice: dts,
	}
}

// NewDataTagsCapped creates a new object with a DataTag slice and its maximum
// capacity.
func NewDataTagsCapped(cap int) *DataTags {
	return &DataTags{
		Slice: make([]DataTag, 0, cap),
	}
}

//func (dts *DataTags) skipInject(current int) bool {
//	//dts.mu.RLock()
//	isAfter := dts.streamPos > 0 && current > dts.streamPos
//	//dts.mu.RUnlock()
//	return isAfter
//}
//
//func (dts *DataTags) incrementLastPost(p int) {
//	//dts.mu.Lock()
//	dts.streamPos += p
//	//dts.mu.Unlock()
//}
//
//func (dts *DataTags) getLastPost() (p int) {
//	//dts.mu.Lock()
//	p = dts.streamPos
//	//dts.mu.Unlock()
//	return
//}

// InjectContent reads from r and uses the data in a Tag to get injected a the
// current position and then writes the output to w. DataTags must be a sorted
// slice. Usually this function receives the data from Entities.QueryResources()
func (dts *DataTags) InjectContent(r io.Reader, w io.Writer) (nRead, nWritten int, _ error) {
	const writeErr = "[esitag] DataTags.InjectContent: Failed to write middleware data to writer: %q for tag index %d with start position %d and end position %d"

	dataBuf := bufpool.Get()
	defer bufpool.Put(dataBuf)
	data := dataBuf.Bytes()

	var prevBufDataSize int
	for di, dt := range dts.Slice {

		bufDataSize := dt.End - prevBufDataSize

		if cap(data) < bufDataSize {
			dataBuf.Grow(bufDataSize - cap(data))
			data = dataBuf.Bytes()
		}
		data = data[:bufDataSize]
		n, err := r.Read(data)
		nRead += n
		if err != nil && err != io.EOF {
			return nRead, nWritten, errors.NewReadFailedf("[esitag] DataTags.InjectContent: Read failed: %q for tag index %d with start position %d and end position %d", err, di, dt.Start, dt.End)
		}
		data = data[:n]
		dts.streamPos += n

		skipInsertion := dt.Start > dts.streamPos || dt.End < dts.streamPos
		fmt.Printf("2:Stream Pos:%d Read:%d Start:%d End:%d skipInsertion:%t => %q\n", dts.streamPos, n, dt.Start, dt.End, skipInsertion, data)

		if skipInsertion {
			// we have not yet reached our position. so write the data to client and continue.
			wn, errW := w.Write(data)
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf(writeErr, errW, di, dt.Start, dt.End)
			}
			continue
		}

		if n > 0 {
			esiStartPos := n - (dt.End - dt.Start)
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
		}
		prevBufDataSize = dt.End
	}

	// io.Copy has more over head than the following code ;-)
	data = data[:cap(data)]
	if len(data) == 0 {
		panic("TODO caddyesi esitag InjectContent should not be possible ;-(")
	}
	for {
		n, err := r.Read(data)
		nRead += n
		if err == io.EOF {
			break // or return here
		}
		if err != nil {
			return nRead, nWritten, errors.NewReadFailedf("[esitag] InjectContent failed to read remaining data: %s", err)
		}
		data = data[:n]

		n, err = w.Write(data)
		nWritten += n
		if err != nil {
			return nRead, nWritten, errors.NewWriteFailedf("[esitag] InjectContent failed to copy remaining data to w: %s", err)
		}
	}

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
