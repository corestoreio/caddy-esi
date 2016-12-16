// +build ignore

// todo: checkout out go4.org/bytereplacer

// All material is licensed under the Apache License Version 2.0, January 2004
// http://www.apache.org/licenses/LICENSE-2.0
//
// Response to Bill's tweet.
// See: https://twitter.com/goinggodotnet/status/760670982617108481
//      https://play.golang.org/p/ykbuIJdoW2
//
// Sample program that takes a stream of bytes and looks for the bytes
// “elvis” and when they are found, replace them with “Elvis”. The code
// cannot assume that there are any line feeds or other delimiters in the
// stream and the code must assume that the stream is of any arbitrary length.
// The solution cannot meaningfully buffer to the end of the stream and
// then process the replacement.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// data represents a table of input and expected output.
var data = []struct {
	input  []byte
	output []byte
}{
	{[]byte("abc"), []byte("abc")},
	{[]byte("elvis"), []byte("Elvis")},
	{[]byte("aElvis"), []byte("aElvis")},
	{[]byte("abcelvis"), []byte("abcElvis")},
	{[]byte("eelvis"), []byte("eElvis")},
	{[]byte("aelvis"), []byte("aElvis")},
	{[]byte("aabeeeelvis"), []byte("aabeeeElvis")},
	{[]byte("e l v i s"), []byte("e l v i s")},
	{[]byte("aa bb e l v i saa"), []byte("aa bb e l v i saa")},
	{[]byte(" elvi s"), []byte(" elvi s")},
	{[]byte("elvielvis"), []byte("elviElvis")},
	{[]byte("elvielvielviselvi1"), []byte("elvielviElviselvi1")},
	{[]byte("elvielviselvis"), []byte("elviElvisElvis")},
}

// Declare what needs to be found and its replacement.
var find = []byte("elvis")
var repl = []byte("Elvis")

func main() {
	var output bytes.Buffer
	// Range over the table testing the algorithm against each input/output.
	for _, d := range data {
		// Use the bytes package to provide a stream to process.
		input := bytes.NewReader(d.input)

		// Create a new replaceReader from the input io.Reader.
		replacer := NewReplaceReader(input, find, repl)

		// Copy from the replacer reader to the output buffer.
		output.Reset()
		if _, err := io.Copy(&output, replacer); err != nil {
			panic(err)
		}

		got, expect := output.Bytes(), d.output

		// Display the results.
		fmt.Printf("Match: %v Inp: [%s] Exp: [%s] Got: [%s]\n", bytes.Compare(got, expect) == 0, d.input, expect, got)
	}
}

// NewReplaceReader returns an io.Reader that reads from r, replacing
// any occurrence of old with new.
func NewReplaceReader(r io.Reader, old, new []byte) io.Reader {
	return &replaceReader{
		br:  bufio.NewReader(r),
		old: old,
		new: new,
	}
}

type replaceReader struct {
	br       *bufio.Reader
	old, new []byte
}

// next returns a slice of translated bytes containing either
// a single byte of unchanged data, or the replacement data
// because a match was found. If an error is returned, the
// slice should not be used.
func (r *replaceReader) next() ([]byte, error) {
	p, err := r.br.Peek(len(r.old))
	if err == nil && bytes.Equal(r.old, p) {
		// A match was found. Advance the bufio reader past the match.
		if _, err := r.br.Discard(len(r.old)); err != nil {
			return nil, err
		}
		return r.new, nil
	}
	// Ignore any peek errors because we may not be able to peek, but
	// still be able to read a byte.
	c, err := r.br.ReadByte()
	if err != nil {
		return nil, err
	}
	return []byte{c}, nil
}

// Read reads into p the translated bytes.
func (r *replaceReader) Read(p []byte) (int, error) {
	var n int
	for {
		// Read reads up to len(p) bytes into p. Since we could potentially add
		// len(r.new) new bytes, we check here that p still has capacity.
		if n+len(r.new) >= len(p) {
			return n, nil
		}

		b, err := r.next()
		if err != nil {
			return n, err
		}
		copy(p[n:], b)
		n += len(b)
	}
}
