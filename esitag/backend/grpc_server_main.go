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

// +build ignore

package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"

	"github.com/corestoreio/caddy-esi/esitag/backend/esigrpc"
	"github.com/corestoreio/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	// also stored in _test file
	serverListenAddr = "127.0.0.1:50049"
)

type server struct{}

func (s server) GetHeaderBody(_ context.Context, arg *esigrpc.ResourceArgs) (*esigrpc.HeaderBody, error) {

	if arg.GetExternalReq() == nil {
		return nil, errors.Empty.Newf("[grpc_server] GetExternalReq cannot be empty")
	}

	if strings.Contains(arg.GetKey(), "error") {
		// If you change the text, a test will fail
		return nil, errors.Interrupted.Newf("[grpc_server] Interrupted. Detected word error in %q for URL %q", arg.GetKey(), arg.GetUrl())
	}

	if strings.HasSuffix(arg.GetKey(), ".html") {
		// http_server_main also reads the file from the disk so stay consitent when
		// running benchmarks.
		cartExampleContent, err := ioutil.ReadFile("testdata/cart_example.html")
		if err != nil {
			return nil, err
		}
		return &esigrpc.HeaderBody{
			Body: cartExampleContent,
		}, nil
	}

	buf := new(bytes.Buffer)
	writeLine(buf, "Arg URL", arg.GetUrl())
	writeLine(buf, "Arg Key", arg.GetKey())
	writeLine(buf, "RequestURI", arg.GetExternalReq().RequestUri)
	writeLine(buf, "Time", time.Now().Format(time.RFC3339))
	writeLine(buf, "BodyEcho", string(arg.GetExternalReq().GetBody()))

	return &esigrpc.HeaderBody{
		// Header: []*esigrpc.MapValues{},
		Body: buf.Bytes(),
	}, nil
}

func writeLine(buf *bytes.Buffer, key, val string) {
	buf.WriteString(`<p>`)
	buf.WriteString(key)
	buf.WriteString(": ")
	buf.WriteString(val)
	buf.WriteString(`</p>`)
	buf.WriteRune('\n')
}

func main() {
	lis, err := net.Listen("tcp", serverListenAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	esigrpc.RegisterHeaderBodyServiceServer(s, server{})

	// Register reflection service on gRPC server.
	// Why the heck do I need this service? => https://github.com/grpc/grpc/blob/master/doc/server-reflection.md
	reflection.Register(s)

	log.Println("Try starting gRPC server ...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
