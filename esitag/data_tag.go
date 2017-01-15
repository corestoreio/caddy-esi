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
	"io"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/corestoreio/errors"
)

// DataTag identifies an ESI tag by its start and end position in the HTML byte
// stream for replacing. If the HTML changes there needs to be a refresh call to
// re-parse the HTML.
type DataTag struct {
	// Data from the micro service gathered in a goroutine.
	Data  []byte
	Start int // Start position in the stream
	End   int // End position in the stream. Never smaller than Start.
}

// DataTags a list of tags with their position within a page and the content
type DataTags []DataTag

// InjectContent reads from r and uses the data in a Tag to get injected a the
// current position and then writes the output to w. DataTags must be a sorted
// slice. Usually this function receives the data from Entities.QueryResources()
func (dts DataTags) InjectContent(r io.Reader, w io.Writer, lastStreamPos int) (nRead, nWritten int, _ error) {
	//if len(dts) == 0 {
	//	println("dts is empty")
	//	return 0, 0, nil
	//}

	// lastStreamPos: first call to InjectContent and the lastStreamPos equals 0
	// because we're at the beginning of the stream. the next call to
	// InjectContent and lastStreamPos gets incremented by the length of the
	// previous data in the Reader. DataTags.InjectContent does not know it gets
	// called multiple times and hence it can inject the ESI tag data in
	// subsequent calls. So lastStreamPos protects recurring replacements.
	// TODO(CyS) implement lastStreamPos

	dataBuf := bufpool.Get()
	defer bufpool.Put(dataBuf)
	data := dataBuf.Bytes()

	var prevBufDataSize int
	for di, dt := range dts {
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

		if n > 0 {
			esiStartPos := n - (dt.End - dt.Start)
			wn, errW := w.Write(data[:esiStartPos])
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf("[esitag] DataTags.InjectContent: Failed to write middleware data to writer: %q for tag index %d with start position %d and end position %d", errW, di, dt.Start, dt.End)
			}

			wn, errW = w.Write(dt.Data)
			nWritten += wn
			if errW != nil {
				return nRead, nWritten, errors.NewWriteFailedf("[esitag] DataTags.InjectContent: Failed to write resource backend data to writer: %q for tag index %d with start position %d and end position %d", errW, di, dt.Start, dt.End)
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
func (dts DataTags) DataLen() (l int) {
	for _, dt := range dts {
		// subtract the length of the raw ESI tag (end-start) from the data
		// length to get the correct length of the inserted data for the
		// Content-Length header. End can never be smaller than Start. The sum
		// can be negative, means the returned data from the backend resource is
		// shorter than the ESI tag itself.
		l += len(dt.Data) - (dt.End - dt.Start)
	}
	return
}

func (dts DataTags) Len() int           { return len(dts) }
func (dts DataTags) Swap(i, j int)      { dts[i], dts[j] = dts[j], dts[i] }
func (dts DataTags) Less(i, j int) bool { return dts[i].Start < dts[j].Start }
