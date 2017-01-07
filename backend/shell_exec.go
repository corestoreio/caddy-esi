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

package backend

import (
	"net/http"
	"os/exec"
	"strings"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/corestoreio/errors"
)

func init() {
	RegisterResourceHandler("sh", NewFetchShellExec())
}

type fetchShellExec struct{}

// NewFetchShellExec creates a new command line executor backend fetcher which
// lives the whole application running time. Thread safe.
func NewFetchShellExec() *fetchShellExec {
	return &fetchShellExec{}
}

// FetchShellExec executes a local program and returns the content or an error.
// Header is always nil.
//
// To trigger shell exec the src attribute in an ESI tag must start with sh://.
//
// If the first character is a white space then we cut thru that white space and
// treat the last characters as arguments.
// 		sh://slow/php/script.php --arg1=1 --arg2=2
//
// This command won't work and creates a weird error:
// 		sh://php slow/php/script.php --arg1=1 --arg2=2
// Fixes welcome!
func (fs *fetchShellExec) DoRequest(args *ResourceArgs) (http.Header, []byte, error) {
	if err := args.Validate(); err != nil {
		return nil, nil, errors.Wrap(err, "[esibackend] FetchShellExec.args.Validate")
	}

	const urlPrefix = `sh://`
	if len(args.URL) <= len(urlPrefix) {
		return nil, nil, errors.NewNotValidf("[esibackend] URL %q not valid. Must start with %q", args.URL, urlPrefix)
	}
	cmdArgs := ""
	cmdName := args.URL[len(urlPrefix):]
	firstWS := strings.IndexAny(cmdName, " \t")
	if firstWS > 0 {
		// this might lead to confusion
		cmdName = cmdName[:firstWS]
		cmdArgs = cmdName[firstWS+1:]
	}

	// remove bufpool and use exec.Cmd pool
	stdErr := bufpool.Get()
	defer bufpool.Put(stdErr)
	stdOut := bufpool.Get()
	defer bufpool.Put(stdOut)
	stdIn := bufpool.Get()
	defer bufpool.Put(stdIn)

	cmd := exec.CommandContext(args.ExternalReq.Context(), cmdName, cmdArgs) // may be use a exec.Cmd pool

	cmd.Stderr = stdErr
	cmd.Stdout = stdOut
	cmd.Stdin = stdIn

	jData, err := args.MarshalJSON()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "[esibackend] FetchShellExec MarshalJSON URL %q", args.URL)
	}
	_, _ = stdIn.Write(jData)

	if err := cmd.Run(); err != nil {
		cmd.Process.Release()
		return nil, nil, errors.Wrapf(err, "[esibackend] FetchShellExec cmd.Run URL %q", args.URL)
	}

	if stdErr.Len() > 0 {
		return nil, nil, errors.NewFatalf("[esibackend] FetchShellExec Process %q error: %q", args.URL, stdErr)
	}

	ret := make([]byte, stdOut.Len())
	copy(ret, stdOut.Bytes())

	return nil, ret, nil
}

// Close noop function
func (fs *fetchShellExec) Close() error {
	return nil
}
