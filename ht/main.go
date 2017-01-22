package main

import (
	"fmt"
	"net/http/httputil"
	"os"
	"runtime"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
)

const caddyAddress = `http://127.0.0.1:2017/`

func main() {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		panic(err)
	}
	c := ht.Collection{
		Tests: testCollection,
	}

	var exitStatus int
	if err := c.ExecuteConcurrent(runtime.NumCPU(), jar); err != nil {
		exitStatus = 26 // line number ;-)
		println("ExecuteConcurrent:", err.Error())
	}

	for _, test := range c.Tests {
		if err := test.PrintReport(os.Stdout); err != nil {
			panic(err)
		}
		if test.Status > ht.Pass {
			exitStatus = 35 // line number ;-)

			reqData, err := httputil.DumpRequest(test.Request.Request, true)
			if err != nil {
				panic(err)
			}
			fmt.Fprintf(os.Stdout, "Request:\n%s\n", reqData)
			resData, err := httputil.DumpResponse(test.Response.Response, false)
			if err != nil {
				panic(err)
			}
			fmt.Fprintf(os.Stdout, "Response:\n%s\nError: %s\n\n", resData, test.Response.BodyErr)
		}
	}

	// Travis CI requires an exit code for the build to fail. Anything not 0
	// will fail the build.
	os.Exit(exitStatus)
}

// RegisterTest adds a set of tests to the collection
func RegisterTest(tests ...*ht.Test) {
	testCollection = append(testCollection, tests...)
}

var testCollection []*ht.Test
