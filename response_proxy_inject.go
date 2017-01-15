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

package caddyesi

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/SchumacherFM/caddyesi/esitag"
)

func responseWrapInjector(chanTags <-chan esitag.DataTags, w http.ResponseWriter) http.ResponseWriter {
	_, cn := w.(http.CloseNotifier)
	_, fl := w.(http.Flusher)
	_, hj := w.(http.Hijacker)
	_, rf := w.(io.ReaderFrom)

	bw := injectingWriter{
		rw:     w,
		tags:   <-chanTags,
		header: make(http.Header),
	}
	if cn && fl && hj && rf {
		return &injectingFancyWriter{bw}
	}
	if fl {
		return &injectingFlushWriter{bw}
	}
	return &bw
}

// injectingWriter wraps a http.ResponseWriter that implements the minimal
// http.ResponseWriter interface.
type injectingWriter struct {
	rw            http.ResponseWriter
	tags          esitag.DataTags
	flushedHeader bool
	wroteHeader   bool
	code          int
	headerMu      sync.Mutex
	header        http.Header
}

// flushHeader recalculates the content-length and flushes the header
func (b *injectingWriter) flushHeader(addContentLength int) {
	if b.flushedHeader {
		return
	}
	b.headerMu.Lock()
	defer b.headerMu.Unlock()

	const clname = "Content-Length"
	clRaw := b.header.Get(clname)
	cl, _ := strconv.Atoi(clRaw) // ignoring that err ... for now
	b.header.Set(clname, strconv.Itoa(cl+addContentLength))

	for k, v := range b.header {
		b.rw.Header()[k] = v
	}
	b.rw.WriteHeader(b.code)
	b.flushedHeader = true
}

func (b *injectingWriter) Header() http.Header {
	return b.header
}

func (b *injectingWriter) WriteHeader(code int) {
	if !b.wroteHeader {
		b.code = code
		b.wroteHeader = true
	}
	b.flushHeader(b.tags.DataLen())
}

// Write does not write to the client instead it writes in the underlying buffer.
func (b *injectingWriter) Write(p []byte) (int, error) {

	// might be buggy on multiple calls to Write()

	buf := bytes.NewBuffer(p)
	_, nw, err := b.tags.InjectContent(buf, b.rw)

	return nw, err
}

type injectingFancyWriter struct {
	injectingWriter
}

func (f *injectingFancyWriter) CloseNotify() <-chan bool {
	cn := f.injectingWriter.rw.(http.CloseNotifier)
	return cn.CloseNotify()
}
func (f *injectingFancyWriter) Flush() {
	fl := f.injectingWriter.rw.(http.Flusher)
	fl.Flush()
}
func (f *injectingFancyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.injectingWriter.rw.(http.Hijacker)
	return hj.Hijack()
}
func (f *injectingFancyWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := f.injectingWriter.rw.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return nil
}

// ReadFrom writes r into the underlying buffer
func (f *injectingFancyWriter) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(&f.injectingWriter, r)
}

var _ http.CloseNotifier = &injectingFancyWriter{}
var _ http.Flusher = &injectingFancyWriter{}
var _ http.Hijacker = &injectingFancyWriter{}
var _ http.Pusher = &injectingFancyWriter{}
var _ io.ReaderFrom = &injectingFancyWriter{}
var _ http.Flusher = &injectingFlushWriter{}

// injectingFlushWriter implements only http.Flusher mostly used
type injectingFlushWriter struct {
	injectingWriter
}

func (f *injectingFlushWriter) Flush() {
	fl := f.injectingWriter.rw.(http.Flusher)
	fl.Flush()
}
