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

// FetchShellExec TODO fetch from a local executable file.
// Header is always nil
func FetchShellExec(args *RequestFuncArgs) (http.Header, []byte, error) {
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
		cmdName = cmdName[:firstWS]
		cmdArgs = cmdName[firstWS+1:]
	}

	stdErr := bufpool.Get()
	defer bufpool.Put(stdErr)
	stdOut := bufpool.Get()
	defer bufpool.Put(stdOut)
	stdIn := bufpool.Get()
	defer bufpool.Put(stdIn)

	// should be also in JSON
	//for hdr, i := args.PrepareForwardHeaders(), 0; i < len(hdr); i = i + 2 {
	//	stdIn.WriteString(hdr[i])
	//	stdIn.WriteByte(':')
	//	stdIn.WriteString(hdr[i+1])
	//	stdIn.WriteByte('\n')
	//}

	cmd := exec.CommandContext(args.ExternalReq.Context(), cmdName, cmdArgs)
	cmd.Stderr = stdErr
	cmd.Stdout = stdOut
	cmd.Stdin = stdIn

	// todo debug log also with stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, errors.Wrapf(err, "[esibackend] FetchShellExec cmd.Run URL %q", args.URL)
	}

	ret := make([]byte, stdOut.Len())
	copy(ret, stdOut.Bytes())

	return nil, ret, nil
}
