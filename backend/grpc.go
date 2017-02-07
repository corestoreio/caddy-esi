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
	"time"

	"github.com/SchumacherFM/caddyesi/backend/esigrpc"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/gavv/monotime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func init() {
	RegisterResourceHandlerFactory("grpc", NewGRPCClient)
}

type grpcClient struct {
	url    string
	con    *grpc.ClientConn
	client esigrpc.HeaderBodyServiceClient
}

// NewGRPCClient creates a new gRPC client resource handler supporting one remote
// server. The following URL parameters are supported: timeout (duration), tls
// (1 for enabled, then the next parameters must be provided), ca_file (file
// containing the CA root cert file), server_host_override (server name used to
// verify the hostname returned by TLS handshake)
// Examples for cfg.URL:
//		grpc://micro.service.tld:9876
//		grpc://micro.service.tld:34567?timeout=20s&tls=1&ca_file=path/to/ca.pem
func NewGRPCClient(cfg *ConfigItem) (ResourceHandler, error) {
	addr, _, params, err := ParseNoSQLURL(cfg.URL)
	if err != nil {
		return nil, errors.NewNotValidf("[esibackend_grpc] Error parsing URL %q => %s", cfg.URL, err)
	}

	opts := make([]grpc.DialOption, 0, 4)

	if to := params.Get("timeout"); to != "" {
		d, err := time.ParseDuration(to)
		if err != nil {
			return nil, errors.NewNotValidf("[esibackend_grpc] Cannot parse timeout %q with error %v", to, err)
		}
		opts = append(opts, grpc.WithTimeout(d), grpc.WithBlock())
	}
	if params.Get("tls") == "1" {
		var sn string
		if sho := params.Get("server_host_override"); sho != "" {
			sn = sho
		}
		var creds credentials.TransportCredentials
		if caFile := params.Get("ca_file"); caFile != "" {
			var err error
			creds, err = credentials.NewClientTLSFromFile(caFile, sn)
			if err != nil {
				return nil, errors.NewFatalf("[esibackend_grpc] Failed to create TLS credentials %v from file %q", err, caFile)
			}
		} else {
			creds = credentials.NewClientTLSFromCert(nil, sn)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, errors.NewFatalf("[esibackend_grpc] Failed to dial: %v", err)
	}

	return &grpcClient{
		url:    cfg.URL,
		con:    conn,
		client: esigrpc.NewHeaderBodyServiceClient(conn),
	}, nil
}

// Closes closes the resource when Caddy restarts or reloads.
func (mc *grpcClient) Close() error {
	return errors.Wrapf(mc.con.Close(), "[esibackend] GRPC connection close error for URL %q", mc.url)
}

// DoRequest returns a value from the field Key in the args argument. Header is
// not supported. Request cancellation through a timeout (when the client
// request gets cancelled) is supported.
func (mc *grpcClient) DoRequest(args *ResourceArgs) (http.Header, []byte, error) {
	timeStart := monotime.Now()

	if err := args.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "[esibackend] FetchHTTP.args.Validate")
	}
	// TODO(CyS) Distinguish between GET and POST like requests to reduce argument
	// building times and allocs.
	in := &esigrpc.ResourceArgs{
		ExternalReq: &esigrpc.ResourceArgs_ExternalReq{
			Method:           args.ExternalReq.Method,
			Url:              args.ExternalReq.URL.String(), // needed? maybe remove
			Proto:            args.ExternalReq.Proto,
			ProtoMajor:       int32(args.ExternalReq.ProtoMajor),
			ProtoMinor:       int32(args.ExternalReq.ProtoMinor),
			Header:           args.PrepareForwardHeaders(),
			ContentLength:    args.ExternalReq.ContentLength,
			TransferEncoding: args.ExternalReq.TransferEncoding,
			Close:            args.ExternalReq.Close,
			Host:             args.ExternalReq.Host,
			Form:             args.PrepareForm(),
			PostForm:         args.PreparePostForm(),
			RemoteAddr:       args.ExternalReq.RemoteAddr,
			RequestUri:       args.ExternalReq.RequestURI,
		},
		Url:              args.URL,
		MaxBodySize:      args.MaxBodySize,
		Key:              args.Key,
		ReturnHeaders:    args.ReturnHeaders,
		ReturnHeadersAll: args.ReturnHeadersAll,
	}

	r, err := mc.client.GetHeaderBody(args.ExternalReq.Context(), in)
	if args.Log.IsDebug() {
		args.Log.Debug("backend.grpcClient.DoRequest.ResourceArg",
			log.Err(err), log.Duration(log.KeyNameDuration, monotime.Since(timeStart)),
			log.Marshal("resource_args", args), log.Object("grpc_resource_args", in),
			log.String("grpc_return_body", string(r.GetBody())),
		)
	}
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esibackend] GetHeaderBody")
	}

	return nil, r.GetBody(), nil
}

func (mc *grpcClient) validateArgs(args *ResourceArgs) (err error) {
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
