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

// +build ignore

// Package main implements a simple server which is used on cyrillschumacher.com
// to demonstrate the usage and performance of the ESI middleware in conjunction
// with gRPC.
package main

import (
	"bytes"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag/backend/esigrpc"
	"github.com/corestoreio/errors"
	"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	serverListenAddr = "127.0.0.1:50055"
)

type server struct {
	session *cache.Cache
}

func (s server) GetHeaderBody(_ context.Context, arg *esigrpc.ResourceArgs) (*esigrpc.HeaderBody, error) {

	if arg.GetExternalReq() == nil {
		return nil, errors.NewEmptyf("[grpc_server] GetExternalReq cannot be empty")
	}

	// key will be set in an ESI tag.
	// <esi:include src="grpcServerdemo" key="session_{Fsession}" timeout="500ms" onerror="Demo gRPC server unavailable :-("/>
	key := arg.GetKey() // key is now "session_JHDASDHASKDH_\x00"
	if len(key) < 20 {
		return nil, errors.NewNotValidf("[grpc_server] Session key %q not valid", key)
	}

	if _, ok := s.session.Get(key); !ok {
		s.session.Set(key, int64(1), 4*time.Minute)
	}
	inc, err := s.session.IncrementInt64(key, 1)
	if err != nil {
		return nil, errors.NewFatalf("[grpc_server] Failed to increment %q", key)
	}

	var buf bytes.Buffer
	buf.WriteString("<table><tr><th>Key</th><th>Value</th></tr>\n")
	writeLine(&buf, "Session", key)
	writeLine(&buf, "Next Integer", strconv.FormatInt(inc, 10))
	writeLine(&buf, "RequestURI", arg.GetExternalReq().RequestUri)
	writeLine(&buf, "Time", time.Now().Format(time.RFC1123Z))
	buf.WriteString("</table>\n")

	//select {
	//case <-ctx.Done():
	//	return nil,errors.NewTimeoutf("[grpc_server] Request timed out: %s",ctx.Err())
	//}

	return &esigrpc.HeaderBody{
		// Header: []*esigrpc.MapValues{},
		Body: buf.Bytes(),
	}, nil
}

func writeLine(buf *bytes.Buffer, key, val string) {
	buf.WriteString(`<tr><td>`)
	buf.WriteString(key)
	buf.WriteString("</td><td>")
	buf.WriteString(val)
	buf.WriteString(`</td></tr>`)
	buf.WriteRune('\n')
}

func main() {

	lis, err := net.Listen("tcp", serverListenAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	esigrpc.RegisterHeaderBodyServiceServer(s, server{
		session: cache.New(5*time.Minute, 30*time.Second),
	})

	log.Printf("Try starting gRPC server on %q", serverListenAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
