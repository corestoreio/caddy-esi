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
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/SchumacherFM/caddyesi/esitag/backend/esigrpc"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/gavv/monotime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func init() {
	esitag.RegisterResourceHandlerFactory("grpc", NewGRPCClient)
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
// Examples for url:
//		grpc://micro.service.tld:9876
//		grpc://micro.service.tld:34567?timeout=20s&tls=1&ca_file=path/to/ca.pem
func NewGRPCClient(opt *esitag.ResourceOptions) (esitag.ResourceHandler, error) {
	addr, _, params, err := opt.ParseNoSQLURL()
	if err != nil {
		return nil, errors.NewNotValidf("[esibackend_grpc] Error parsing URL %q => %s", opt.URL, err)
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
		url:    opt.URL,
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
func (mc *grpcClient) DoRequest(args *esitag.ResourceArgs) (http.Header, []byte, error) {
	timeStart := monotime.Now()

	if err := args.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "[esibackend] gRPC.args.Validate")
	}
	if err := args.ValidateWithKey(); err != nil {
		return nil, nil, errors.Wrap(err, "[esibackend] gRPC.args.ValidateWithKey")
	}

	r := args.ExternalReq
	var body []byte
	if args.IsPostAllowed() {
		var err error
		body, err = ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			return nil, nil, errors.NewReadFailedf("[esibackend] Body too large: %s", err)
		}
		r.Body = ioutil.NopCloser(bytes.NewReader(body))
	}

	in := &esigrpc.ResourceArgs{
		ExternalReq: &esigrpc.ResourceArgs_ExternalReq{
			Method:           r.Method,
			Url:              r.URL.String(), // needed? maybe remove
			Proto:            r.Proto,
			ProtoMajor:       int32(r.ProtoMajor),
			ProtoMinor:       int32(r.ProtoMinor),
			Header:           args.PrepareForwardHeaders(),
			ContentLength:    r.ContentLength,
			TransferEncoding: r.TransferEncoding,
			Close:            r.Close,
			Host:             r.Host,
			RemoteAddr:       r.RemoteAddr,
			RequestUri:       r.RequestURI,
			Body:             body,
		},
		Url:              args.URL,
		MaxBodySize:      args.Tag.MaxBodySize,
		Key:              args.Tag.Key,
		ReturnHeaders:    args.Tag.ReturnHeaders,
		ReturnHeadersAll: args.Tag.ReturnHeadersAll,
	}

	hb, err := mc.client.GetHeaderBody(r.Context(), in)
	if args.Tag.Log.IsDebug() {
		args.Tag.Log.Debug("backend.grpcClient.DoRequest.ResourceArg",
			log.Err(err), log.Duration(log.KeyNameDuration, monotime.Since(timeStart)),
			log.Marshal("resource_args", args), log.Object("grpc_resource_args", in),
			log.String("grpc_return_body", string(hb.GetBody())),
		)
	}
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esibackend] GetHeaderBody")
	}

	bdy := hb.GetBody()
	if mbs := int(args.Tag.MaxBodySize); len(bdy) > mbs && mbs > 0 {
		bdy = bdy[:mbs]
	}
	return nil, bdy, nil
}
