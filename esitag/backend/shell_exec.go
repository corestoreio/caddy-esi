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

// +build esiall esishell

package backend

import (
	"context"
	"net/http"
	"os/exec"
	"strings"

	"github.com/corestoreio/caddy-esi/bufpool"
	"github.com/corestoreio/caddy-esi/esitag"
	"github.com/corestoreio/errors"
)

func init() {
	esitag.RegisterResourceHandler("sh", NewFetchShellExec())
}

type fetchShellExec struct{}

// NewFetchShellExec creates a new command line executor backend fetcher which
// lives the whole application running time. Thread safe. Slow.
func NewFetchShellExec() esitag.ResourceHandler {
	return &fetchShellExec{}
}

// DoRequest executes a local program and returns the content or an error.
// Header is always nil.
//
// To trigger shell exec the src attribute in an Tag tag must start with sh://.
//
// If the first character is a white space then we cut thru that white space and
// treat the last characters as arguments.
// 		sh://slow/php/script.php --arg1=1 --arg2=2
//
// This command won't work and creates a weird error:
// 		sh://php slow/php/script.php --arg1=1 --arg2=2
// Fixes welcome!
func (fs *fetchShellExec) DoRequest(args *esitag.ResourceArgs) (http.Header, []byte, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "[esibackend] FetchShellExec.args.Validate")
	}

	const urlPrefix = `sh://`
	if len(args.URL) <= len(urlPrefix) {
		return nil, nil, errors.NotValid.Newf("[esibackend] URL %q not valid. Must start with %q", args.URL, urlPrefix)
	}
	cmdArgs := ""
	cmdName := args.URL[len(urlPrefix):]
	firstWS := strings.IndexAny(cmdName, " \t")
	if firstWS > 0 && len(cmdName) >= firstWS+1 {
		cmdArgs = cmdName[firstWS+1:]
		cmdName = cmdName[:firstWS]
	}

	// The overhead of the goroutine and the channel is negligible and worth to
	// rely on "non-blocking" execution, the channel blocks ;-)
	var retContent []byte
	var retErr error
	done := make(chan struct{})
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		// remove bufpool and use exec.Cmd pool
		stdErr := bufpool.Get()
		defer bufpool.Put(stdErr)
		stdOut := bufpool.Get()
		defer bufpool.Put(stdOut)
		stdIn := bufpool.Get()
		defer bufpool.Put(stdIn)

		ctx, cancel := context.WithTimeout(args.ExternalReq.Context(), args.Tag.Timeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, cmdName, cmdArgs) // may be use a exec.Cmd pool

		cmd.Stderr = stdErr
		cmd.Stdout = stdOut
		cmd.Stdin = stdIn

		jData, err := args.MarshalJSON()
		if err != nil {
			retErr = errors.Wrapf(err, "[esibackend] FetchShellExec MarshalJSON URL %q", args.URL)
			return
		}
		_, _ = stdIn.Write(jData)

		if err := cmd.Run(); err != nil {
			retErr = errors.Wrapf(err, "[esibackend] FetchShellExec cmd.Run URL %q", args.URL)
			return
		}

		if stdErr.Len() > 0 {
			retErr = errors.Fatal.Newf("[esibackend] FetchShellExec Process %q error: %q", args.URL, stdErr)
			return
		}
		ln := stdOut.Len()
		if mbs := int(args.Tag.MaxBodySize); ln > mbs && mbs > 0 {
			ln = mbs
		}
		retContent = make([]byte, ln)
		n := copy(retContent, stdOut.Bytes())
		retContent = retContent[:n]
	}()
	<-done
	return nil, retContent, errors.Wrap(retErr, "[esibackend] Returned from error")
}

// Close noop function
func (fs *fetchShellExec) Close() error {
	return nil
}
