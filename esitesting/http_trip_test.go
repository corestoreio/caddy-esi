package esitesting_test

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/corestoreio/caddy-esi/esitesting"
	"github.com/corestoreio/errors"
	"github.com/stretchr/testify/assert"
)

var _ http.RoundTripper = (*esitesting.HTTPTrip)(nil)

func TestNewHttpTrip_Ok(t *testing.T) {
	t.Parallel()
	tr := esitesting.NewHTTPTrip(333, "Hello Wørld", nil)
	cl := &http.Client{
		Transport: tr,
	}
	const reqURL = `http://corestore.io`
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			getReq, err := http.NewRequest("GET", reqURL, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := cl.Do(getReq)
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Fatal(err)
				}
			}()
			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			assert.Exactly(t, "Hello Wørld", string(data))
			assert.Exactly(t, 333, resp.StatusCode)
		}(&wg)
	}
	wg.Wait()

	tr.RequestsMatchAll(t, func(r *http.Request) bool {
		return r.URL.String() == reqURL
	})
	tr.RequestsCount(t, 10)
}

func TestNewHttpTrip_Error(t *testing.T) {
	t.Parallel()
	tr := esitesting.NewHTTPTrip(501, "Hello Error", errors.NotValid.Newf("test not valid"))
	cl := &http.Client{
		Transport: tr,
	}

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(wg *sync.WaitGroup) {
			defer wg.Done()
			getReq, err := http.NewRequest("GET", "http://noophole.com", nil)
			if err != nil {
				t.Fatal("NewRequest", err)
			}
			resp, err := cl.Do(getReq)
			assert.True(t, errors.IsNotValid(err.(*url.Error).Err), "ErrorDo: %#v", err)
			assert.Nil(t, resp)
		}(&wg)
	}
	wg.Wait()
	tr.RequestsCount(t, 10)
}
