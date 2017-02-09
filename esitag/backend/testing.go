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
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/SchumacherFM/caddyesi/esitag"
	"github.com/corestoreio/errors"
	ps "github.com/mitchellh/go-ps"
)

func init() {
	esitag.RegisterResourceHandlerFactory("mockTimeout", func(opt *esitag.ResourceOptions) (esitag.ResourceHandler, error) {
		return ResourceMock{
			DoRequestFn: func(*esitag.ResourceArgs) (_ http.Header, content []byte, err error) {
				// mockTimeout://duration
				return nil, nil, errors.NewTimeoutf("[backend] Timeout after %q", opt.URL)
			},
		}, nil
	})
}

const mockRequestMsg = "%s %q Timeout %s MaxBody %s"

// ResourceMock exported for testing
type ResourceMock struct {
	DoRequestFn func(args *esitag.ResourceArgs) (http.Header, []byte, error)
	CloseFn     func() error
}

// DoRequest calls DoRequestFn
func (rm ResourceMock) DoRequest(a *esitag.ResourceArgs) (http.Header, []byte, error) {
	return rm.DoRequestFn(a)
}

// Close returns nil if CloseFn is nil otherwise calls CloseFn
func (rm ResourceMock) Close() error {
	if rm.CloseFn == nil {
		return nil
	}
	return rm.CloseFn()
}

// MockRequestContent for testing purposes only.
func MockRequestContent(content string) esitag.ResourceHandler {
	return ResourceMock{
		DoRequestFn: func(args *esitag.ResourceArgs) (http.Header, []byte, error) {
			if args.URL == "" && args.Key == "" {
				panic(fmt.Sprintf("[esibackend] URL and Key cannot be empty: %#v\n", args))
			}
			return nil, []byte(fmt.Sprintf(mockRequestMsg, content, args.URL, args.Timeout, args.MaxBodySizeHumanized())), nil
		},
	}
}

// MockRequestContentCB for testing purposes only. Call back gets executed
// before the function returns.
func MockRequestContentCB(content string, callback func() error) esitag.ResourceHandler {
	return ResourceMock{
		DoRequestFn: func(args *esitag.ResourceArgs) (http.Header, []byte, error) {
			if err := callback(); err != nil {
				return nil, nil, errors.Wrapf(err, "MockRequestContentCB with URL %q", args.URL)
			}
			return nil, []byte(fmt.Sprintf(mockRequestMsg, content, args.URL, args.Timeout, args.MaxBodySizeHumanized())), nil
		},
	}
}

// MockRequestError for testing purposes only.
func MockRequestError(err error) esitag.ResourceHandler {
	return ResourceMock{
		DoRequestFn: func(_ *esitag.ResourceArgs) (http.Header, []byte, error) {
			return nil, nil, err
		},
	}
}

// MockRequestPanic just panics
func MockRequestPanic(msg interface{}) esitag.ResourceHandler {
	return ResourceMock{
		DoRequestFn: func(_ *esitag.ResourceArgs) (http.Header, []byte, error) {
			panic(msg)
		},
	}
}

// StartProcess starts a process and returns a cleanup function which kills the
// process. Panics on error.
func StartProcess(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	if err := cmd.Start(); err != nil {
		panic(fmt.Sprintf("skipping test; couldn't find go binary: %v", err))
	}
	return cmd
}

// KillZombieProcess searches a running process by its name and kills it. Writes
// the success to Stderr. Panics on errors.
func KillZombieProcess(processName16 string) {
	pses, err := ps.Processes()
	if err != nil {
		panic(err)
	}
	for _, p := range pses {
		if strings.Contains(p.Executable(), processName16) { // max length 16
			proc, err := os.FindProcess(p.Pid())
			if err != nil {
				panic(err)
			}
			if err := proc.Kill(); err != nil {
				panic(err)
			}
			fmt.Fprintf(os.Stderr, "Killed previous running process %s with pid %d\n", p.Executable(), p.Pid())
		}
	}
}
