// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
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

// build ignore

package main

import (
	"bytes"
	"log"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/corestoreio/caddy-esi/esitag/backend/esigrpc"
	"github.com/corestoreio/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	// also stored in CaddyfileResources.xml file with key grpc_integration_01
	serverListenAddr = "127.0.0.1:50666"
)

type server struct {
	coaOn  uint64
	coaOff uint64
}

func (s *server) GetHeaderBody(_ context.Context, arg *esigrpc.ResourceArgs) (*esigrpc.HeaderBody, error) {

	if arg.GetExternalReq() == nil {
		return nil, errors.NewEmptyf("[grpc_server] GetExternalReq cannot be empty")
	}

	var counter uint64
	switch {
	case arg.GetKey() == "coalesce_enabled":
		counter = atomic.AddUint64(&s.coaOn, 1)
		time.Sleep(300 * time.Millisecond) // long running operation, like a tiny PHP script.
	case arg.GetKey() == "coalesce_disabled":
		counter = atomic.AddUint64(&s.coaOff, 1)
	case strings.Contains(arg.GetKey(), "error"):
		return nil, errors.NewInterruptedf("[grpc_server] Interrupted. Detected word error in %q for URL %q", arg.GetKey(), arg.GetUrl())
	}

	buf := new(bytes.Buffer)
	writeLine(buf, "Arg URL", arg.GetUrl())
	writeLine(buf, "Arg Key", arg.GetKey())
	writeLine(buf, arg.GetKey(), strconv.FormatUint(counter, 10))
	writeLine(buf, "Time", time.Now().Format(time.RFC3339))

	return &esigrpc.HeaderBody{
		// Header: []*esigrpc.MapValues{},
		Body: buf.Bytes(),
	}, nil
}

func writeLine(buf *bytes.Buffer, key, val string) {
	buf.WriteString(`<p>`)
	buf.WriteString(key)
	buf.WriteString("=")
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
	esigrpc.RegisterHeaderBodyServiceServer(s, &server{})

	log.Println("Try starting gRPC server ...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
