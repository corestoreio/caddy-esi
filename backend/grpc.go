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

// +build esiall esigrpc

//go:generate protoc -I esigrpc/ ./esigrpc/grpc_data.proto --go_out=plugins=grpc:esigrpc

package backend

import (
	"net/http"

	"github.com/corestoreio/errors"
)

func init() {
	RegisterResourceHandlerFactory("grpc", NewGRPC)
}

type esiGRPC struct {
	isCancellable bool
	url           string
}

// NewGRPC creates a new memcache resource handler supporting n-memcache server.
func NewGRPC(cfg *ConfigItem) (ResourceHandler, error) {

	// TODO init grpc client

	return nil, nil
}

// Closes closes the resource when Caddy restarts or reloads. If supported
// by the resource.
func (mc *esiGRPC) Close() error {
	return nil
}

// DoRequest returns a value from the field Key in the args argument. Header is
// not supported. Request cancellation through a timeout (when the client
// request gets cancelled) is supported.
func (mc *esiGRPC) DoRequest(args *ResourceArgs) (_ http.Header, _ []byte, err error) {

	return nil, nil, nil
}

func (mc *esiGRPC) validateArgs(args *ResourceArgs) (err error) {
	switch {
	case args.Key == "":
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %#v the URL value is empty", args)
	case args.ExternalReq == nil:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q => %q the ExternalReq value is nil", mc.url, args.Key)
	case args.Timeout < 1:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q => %q the timeout value is empty", mc.url, args.Key)
	case args.MaxBodySize == 0:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q => %q the maxBodySize value is empty", mc.url, args.Key)
	}
	return
}
