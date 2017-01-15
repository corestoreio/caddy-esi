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
	rw              http.ResponseWriter
	tags            esitag.DataTags
	responseAllowed uint8 // 0 not yet tested, 1 yes, 2 no
	wroteHeader     bool
	header          http.Header
	lastWritePos    int
}

func (b *injectingWriter) Header() http.Header {
	return b.header
}

func (b *injectingWriter) WriteHeader(code int) {
	if b.wroteHeader {
		return
	}
	b.wroteHeader = true
	dataTagLen := b.tags.DataLen()

	if dataTagLen != 0 {
		const clName = "Content-Length"
		clRaw := b.header.Get(clName)
		cl, _ := strconv.Atoi(clRaw) // ignoring that err ... for now
		// What if cl runs negative?
		b.header.Set(clName, strconv.Itoa(cl+dataTagLen))
	}

	for k, v := range b.header {
		b.rw.Header()[k] = v
	}
	b.rw.WriteHeader(code)
}

// Write does not write to the client instead it writes in the underlying buffer.
func (b *injectingWriter) Write(p []byte) (int, error) {
	const (
		notTested uint8 = iota
		yes
		no
	)

	if b.responseAllowed == notTested {
		// Only plain text response is benchIsResponseAllowed, so detect content type.
		// Hopefully p is longer than 512 bytes ;-)
		b.responseAllowed = yes
		if !isResponseAllowed(p) {
			b.responseAllowed = no
		}
	}

	if b.responseAllowed == no {
		return b.rw.Write(p)
	}

	// might be buggy in InjectContent on multiple calls to Write(). Fix is to a position counter to the InjectContent.
	// The position is pos+=len(p)
	buf := bytes.NewBuffer(p) // todo simplify and use our own type
	_, nw, err := b.tags.InjectContent(buf, b.rw, b.lastWritePos)
	b.lastWritePos += len(p)
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

// injectingFlushWriter implements only http.Flusher mostly used
type injectingFlushWriter struct {
	injectingWriter
}

func (f *injectingFlushWriter) Flush() {
	fl := f.injectingWriter.rw.(http.Flusher)
	fl.Flush()
}
