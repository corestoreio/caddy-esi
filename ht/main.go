package main

import (
	"os"
	"runtime"

	"github.com/vdobler/ht/cookiejar"
	"github.com/vdobler/ht/ht"
)

func main() {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		panic(err)
	}
	c := ht.Collection{
		Tests: []*ht.Test{
			page01(),
			page01(),
			page01(),
		},
	}

	var exitStatus int
	if err := c.ExecuteConcurrent(runtime.NumCPU(), jar); err != nil {
		exitStatus = 24 // line number ;-)
		println("ExecuteConcurrent:", err.Error())
	}

	for _, test := range c.Tests {
		if err := test.PrintReport(os.Stdout); err != nil {
			panic(err)
		}
	}

	// Travis CI requires an exit code for the build to fail. Anything not 0
	// will fail the build.
	os.Exit(exitStatus)
}
