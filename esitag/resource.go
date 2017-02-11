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

package esitag

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	loghttp "github.com/corestoreio/log/http"
	"github.com/dustin/go-humanize"
)

// DefaultTimeOut defines the default timeout to a backend resource.
const DefaultTimeOut = 20 * time.Second

// ResourceArgs arguments to ResourceHandler.DoRequest. This type lives in a
// sync.Pool and the fields ExternalReq, repl and URL gets reset when putting it
// back into the pool.
type ResourceArgs struct {
	// ExternalReq external request object from the evil internet.
	ExternalReq *http.Request
	// repl replaces the template variables of URL and Tag.Key with values form
	// field ExternalReq.
	repl Replacer
	// URL defines the target URL to call or an alias name. This field gets set
	// from type Resource.url in the function Resource.DoRequest. Before passing
	// to DoRequest of the underlying implementation URL gets treated with
	// Replacer.
	URL string
	// Tag the configuration of a single ESI tag.
	Tag Config
}

// NewResourceArgs creates a new argument and initializes the internal string
// replacer.
func NewResourceArgs(r *http.Request, url string, esi Config) *ResourceArgs {
	return &ResourceArgs{
		ExternalReq: r,
		URL:         url,
		Tag:         esi,
		repl:        MakeReplacer(r, ""),
	}
}

// ReplaceKeyURLForTesting only used for integration tests in the backend
// package.
func (a *ResourceArgs) ReplaceKeyURLForTesting() *ResourceArgs {
	a.Tag.Key = a.repl.Replace(a.Tag.Key)
	a.URL = a.repl.Replace(a.URL)
	return a
}

// Validate checks if required arguments have been set
func (a *ResourceArgs) Validate() (err error) {
	switch {
	case a.URL == "":
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %#v the URL value is empty", a)
	case a.ExternalReq == nil:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the ExternalReq value is nil", a.URL)
	case a.Tag.Timeout < 1:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the timeout value is empty", a.URL)
	case a.Tag.MaxBodySize == 0:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the maxBodySize value is empty", a.URL)
	}
	return
}

func (a *ResourceArgs) ValidateWithKey() (err error) {
	switch {
	case a.Tag.Key == "":
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %#v the Key value is empty", a)
	case a.ExternalReq == nil:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the ExternalReq value is nil", a.Tag.Key)
	case a.Tag.Timeout < 1:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the timeout value is empty", a.Tag.Key)
	case a.Tag.MaxBodySize == 0:
		err = errors.NewEmptyf("[esibackend] For ResourceArgs %q the maxBodySize value is empty", a.Tag.Key)
	}
	return
}

// MaxBodySizeHumanized converts the bytes into a human readable format
func (a *ResourceArgs) MaxBodySizeHumanized() string {
	return humanize.Bytes(a.Tag.MaxBodySize)
}

// DropHeadersForward a list of headers which should never be forwarded to the
// backend resource. Initial idea of excluded fields.
// https://en.wikipedia.org/wiki/List_of_HTTP_header_fields
var DropHeadersForward = map[string]bool{
	"Cache-Control": true,
	"Connection":    true,
	"Host":          true,
	"Pragma":        true,
	"Upgrade":       true,
}

// DropHeadersReturn a list of headers which should never be forwarded to the
// client. Initial idea of excluded fields.
// https://en.wikipedia.org/wiki/List_of_HTTP_header_fields
var DropHeadersReturn = map[string]bool{
	"Cache-Control":             true,
	"Connection":                true,
	"Content-Disposition":       true,
	"Content-Encoding":          true,
	"Content-Length":            true,
	"Content-Range":             true,
	"Content-Type":              true,
	"Date":                      true,
	"Etag":                      true,
	"Expires":                   true,
	"Last-Modified":             true,
	"Location":                  true,
	"Status":                    true,
	"Strict-Transport-Security": true,
	"Trailer":                   true,
	"Transfer-Encoding":         true,
	"Upgrade":                   true,
}

// PrepareForm prepares (GET) form values in a balanced slice: i == key and i+1
// == value. A key can occur multiple times. Errors parsing the form gets logged
// on Info level.
func (a *ResourceArgs) PrepareForm() []string {
	if err := a.ExternalReq.ParseForm(); err != nil {
		a.Tag.Log.Info("backend.ResourceArgs.PrepareForm.ExternalReq.ParseForm",
			log.Err(err), loghttp.Request("request", a.ExternalReq), log.Marshal("resource_args", a))
	}

	form := a.ExternalReq.Form

	if !a.Tag.ForwardPostData && a.ExternalReq.URL != nil {
		// if disabled then parse only GET parameters
		var err error
		form, err = url.ParseQuery(a.ExternalReq.URL.RawQuery)
		if err != nil {
			a.Tag.Log.Info("backend.ResourceArgs.PrepareForm.ExternalReq.ParseQuery",
				log.Err(err), loghttp.Request("request", a.ExternalReq), log.Marshal("resource_args", a))
			return nil
		}
		if len(form) == 0 {
			return nil
		}
	}

	ret := make([]string, 0, len(form))
	for k, vals := range form {
		for _, val := range vals {
			ret = append(ret, k, val)
		}
	}
	return ret
}

// PreparePostForm prepares (POST,PUT,PATCH) form values in a balanced slice: i
// == key and i+1 == value. A key can occur multiple times. Errors parsing the
// form gets logged on Info level and will return nil. Does not consider GET
// params.
func (a *ResourceArgs) PreparePostForm() []string {
	if !a.Tag.ForwardPostData {
		return nil
	}

	if err := a.ExternalReq.ParseForm(); err != nil {
		a.Tag.Log.Info("backend.ResourceArgs.PreparePostForm.ExternalReq.ParseForm",
			log.Err(err), loghttp.Request("request", a.ExternalReq), log.Marshal("resource_args", a))
		return nil
	}

	var ret []string
	if lpf := len(a.ExternalReq.PostForm); lpf > 0 {
		ret = make([]string, 0, lpf)
	}

	for k, vals := range a.ExternalReq.PostForm {
		for _, val := range vals {
			ret = append(ret, k, val)
		}
	}
	return ret
}

// PrepareForwardHeaders returns all headers which must get forwarded to the
// backend resource. Returns a non-nil slice when no headers should be
// forwarded. Slice is balanced: i == key and i+1 == value
func (a *ResourceArgs) PrepareForwardHeaders() []string {
	if !a.Tag.ForwardHeadersAll && len(a.Tag.ForwardHeaders) == 0 {
		return []string{}
	}
	if a.Tag.ForwardHeadersAll {
		ret := make([]string, 0, len(a.ExternalReq.Header))
		for hn, hvs := range a.ExternalReq.Header {
			if !DropHeadersForward[hn] {
				for _, hv := range hvs {
					ret = append(ret, hn, hv)
				}
			}
		}
		return ret
	}

	ret := make([]string, 0, len(a.Tag.ForwardHeaders))
	for _, hn := range a.Tag.ForwardHeaders {
		if hvs, ok := a.ExternalReq.Header[hn]; ok && !DropHeadersForward[hn] {
			for _, hv := range hvs {
				ret = append(ret, hn, hv)
			}
		}
	}
	return ret
}

// PrepareReturnHeaders extracts the required headers fromBE as defined in the
// struct fields ReturnHeaders*. fromBE means: From Back End. These are the
// headers from the queried backend resource. Might return a nil map.
func (a *ResourceArgs) PrepareReturnHeaders(fromBE http.Header) http.Header {
	if !a.Tag.ReturnHeadersAll && len(a.Tag.ReturnHeaders) == 0 {
		return nil
	}

	ret := make(http.Header) // using len(fromBE) as 2nd a makes the benchmark slower!
	if a.Tag.ReturnHeadersAll {
		for hn, hvs := range fromBE {
			if !DropHeadersReturn[hn] {
				for _, hv := range hvs {
					ret.Set(hn, hv)
				}
			}
		}
		return ret
	}

	for _, hn := range a.Tag.ReturnHeaders {
		if hvs, ok := fromBE[hn]; ok && !DropHeadersReturn[hn] {
			for _, hv := range hvs {
				ret.Set(hn, hv)
			}
		}
	}
	return ret
}

// MarshalLog special crafted log format, does not log the external request
func (a *ResourceArgs) MarshalLog(kv log.KeyValuer) error {
	kv.AddString("ra_url", a.URL)
	kv.AddInt64("ra_timeout", a.Tag.Timeout.Nanoseconds())
	kv.AddUint64("ra_max_body_size", a.Tag.MaxBodySize)
	kv.AddString("ra_max_body_size_h", a.MaxBodySizeHumanized())
	kv.AddString("ra_key", a.Tag.Key)
	kv.AddInt64("ra_ttl", a.Tag.TTL.Nanoseconds())
	kv.AddBool("ra_forward_post_data", a.Tag.ForwardPostData)
	kv.AddString("ra_forward_headers", strings.Join(a.Tag.ForwardHeaders, "|"))
	kv.AddBool("ra_forward_headers_all", a.Tag.ForwardHeadersAll)
	kv.AddString("ra_return_headers", strings.Join(a.Tag.ReturnHeaders, "|"))
	kv.AddBool("ra_return_headers_all", a.Tag.ReturnHeadersAll)
	return nil
}

var rhRegistry = &struct {
	sync.RWMutex
	handlers map[string]ResourceHandler
}{
	handlers: make(map[string]ResourceHandler),
}

// RegisterResourceHandler scheme can be a protocol before the :// but also an
// alias to register a key-value service. This function returns a closure which
// lets you deregister the scheme/alias once a test has finished. Use the defer
// word. Scheme/alias will be transformed into an all lowercase string.
func RegisterResourceHandler(scheme string, f ResourceHandler) struct{ DeferredDeregister func() } {
	scheme = strings.ToLower(scheme)
	rhRegistry.Lock()
	defer rhRegistry.Unlock()
	rhRegistry.handlers[scheme] = f
	return struct {
		DeferredDeregister func()
	}{
		DeferredDeregister: func() { DeregisterResourceHandler(scheme) },
	}
}

// DeregisterResourceHandler removes a previously registered scheme/alias.
func DeregisterResourceHandler(scheme string) {
	scheme = strings.ToLower(scheme)
	rhRegistry.Lock()
	defer rhRegistry.Unlock()
	delete(rhRegistry.handlers, scheme)
}

// LookupResourceHandler uses the scheme/alias to retrieve the backend request
// function.
func LookupResourceHandler(scheme string) (rf ResourceHandler, ok bool) {
	scheme = strings.ToLower(scheme)
	rhRegistry.RLock()
	defer rhRegistry.RUnlock()
	rf, ok = rhRegistry.handlers[scheme]
	return
}

// CloseAllResourceHandler does what the function name says returns the first
// occurred error.
func CloseAllResourceHandler() error {
	rhRegistry.Lock()
	defer rhRegistry.Unlock()
	for scheme, rh := range rhRegistry.handlers {
		if err := rh.Close(); err != nil {
			return errors.Wrapf(err, "[esibackend] Failed to close %q", scheme)
		}
	}
	return nil
}

var factoryResourceHandlers = &struct {
	sync.RWMutex
	factories map[string]ResourceHandlerFactoryFunc
}{
	factories: make(map[string]ResourceHandlerFactoryFunc),
}

// ResourceOptions has the same structure as caddyesi.ResourceItem. Defines a
// single configuration item for creating a new backend resource, especially in
// ResourceHandlerFactoryFunc.
type ResourceOptions struct {
	// Alias ,optional, can have any name which gets used in an Tag tag and
	// refers to the connection to a resource.
	Alias string
	// URL, required, defines the authentication and target to a resource. If an
	// URL contains the name of an Alias then the URl data from that alias will
	// be copied into this URL field.
	URL string
	// Query, optional, contains mostly a SQL query which runs as a prepared
	// statement so you must use the question mark or any other placeholder.
	Query string
}

// NewResourceOptions creates a new option. URL is mandatory but alias and query
// are optional. Up to three arguments in total are supported.
func NewResourceOptions(url string, aliasQuery ...string) *ResourceOptions {
	ci := &ResourceOptions{
		URL: url,
	}
	switch len(aliasQuery) {
	case 1:
		ci.Alias = aliasQuery[0]
	case 2:
		ci.Alias = aliasQuery[0]
		ci.Query = aliasQuery[1]
	}
	return ci
}

// defaultPoolConnectionParameters this var also exists in the test file
var defaultPoolConnectionParameters = [...]string{
	"db", "0",
	"max_active", "10",
	"max_idle", "400",
	"idle_timeout", "240s",
	"cancellable", "0",
	"lazy", "0", // if 1 disables the ping to redis during caddy startup
}

// ParseNoSQLURL parses a given URL using a custom URI scheme.
// For example:
// 		redis://localhost:6379/?db=3
// 		memcache://localhost:1313/?lazy=1
// 		redis://:6380/?db=0 => connects to localhost:6380
// 		redis:// => connects to localhost:6379 with DB 0
// 		memcache:// => connects to localhost:11211
//		memcache://?server=localhost:11212&server=localhost:11213
//			=> connects to: localhost:11211, localhost:11212, localhost:11213
// 		redis://empty:myPassword@clusterName.xxxxxx.0001.usw2.cache.amazonaws.com:6379/?db=0
// Available parameters: db, max_active (int, Connections), max_idle (int,
// Connections), idle_timeout (time.Duration, Connection), cancellable (0,1
// request towards redis), lazy (0, 1 disables ping during connection setup). On
// successful parse the key "scheme" is also set in the params return value.
func (ro *ResourceOptions) ParseNoSQLURL() (address, password string, params url.Values, err error) {
	u, err := url.Parse(ro.URL)
	if err != nil {
		return "", "", nil, errors.Errorf("[backend] url.Parse: %s", err)
	}

	host, port, err := net.SplitHostPort(u.Host)
	if sErr, ok := err.(*net.AddrError); ok && sErr != nil && sErr.Err == "too many colons in address" {
		return "", "", nil, errors.Errorf("[backend] SplitHostPort: %s", err)
	}
	if err != nil {
		// assume port is missing
		host = u.Host
		if port == "" {
			switch u.Scheme {
			case "redis":
				port = "6379"
			case "memcache":
				port = "11211"
			default:
				// might leak password because raw URL gets output ...
				return "", "", nil, errors.NewNotSupportedf("[backend] ParseNoSQLURL unsupported scheme %q because Port is empty. URL: %q", u.Scheme, ro.URL)
			}
		}
	}
	if host == "" {
		host = "localhost"
	}
	address = net.JoinHostPort(host, port)

	if u.User != nil {
		password, _ = u.User.Password()
	}

	params, err = url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", "", nil, errors.NewNotValidf("[backend] ParseNoSQLURL: Failed to parse %q for parameters in URL %q with error %s", u.RawQuery, ro.URL, err)
	}

	for i := 0; i < len(defaultPoolConnectionParameters); i = i + 2 {
		if params.Get(defaultPoolConnectionParameters[i]) == "" {
			params.Set(defaultPoolConnectionParameters[i], defaultPoolConnectionParameters[i+1])
		}
	}
	params.Set("scheme", u.Scheme)

	return
}

// ResourceHandlerFactoryFunc can create a new resource handler or an error.
type ResourceHandlerFactoryFunc func(*ResourceOptions) (ResourceHandler, error)

// RegisterResourceHandlerFactory registers a new factory function to create a
// new ResourceHandler. Useful when you have entries in the
// resources_config.xml|json file.
func RegisterResourceHandlerFactory(scheme string, f ResourceHandlerFactoryFunc) {
	factoryResourceHandlers.Lock()
	factoryResourceHandlers.factories[scheme] = f
	factoryResourceHandlers.Unlock()
}

func newResourceHandlerFromFactory(scheme string, opt *ResourceOptions) (ResourceHandler, error) {
	factoryResourceHandlers.RLock()
	defer factoryResourceHandlers.RUnlock()
	if f, ok := factoryResourceHandlers.factories[scheme]; ok && scheme != "" {
		return f(opt)
	}
	return nil, errors.NewNotSupportedf("[backend] Alias %q not supported in factory registry", scheme)
}

// ResourceHandler gets implemented by any client which is able to speak to any
// remote backend. A handler gets registered in a global map and has a long
// lived state.
type ResourceHandler interface {
	// DoRequest fires the request to the resource and it may return a header
	// and content or an error. All three return values can be nil. An error can
	// have the behaviour of NotFound which calls the next resource in the
	// sequence and does not trigger the circuit breaker. Any other returned
	// error will trigger the increment of the circuit breaker. See the variable
	// CBMaxFailures for the maximum amount of allowed failures until the
	// circuit breaker opens.
	DoRequest(*ResourceArgs) (header http.Header, content []byte, err error)
	// Closes closes the resource when Caddy restarts or reloads. If supported
	// by the resource.
	Close() error
}

// NewResourceHandler a given URL gets checked which service it should
// instantiate and connect to. A handler must be previously registered via
// function RegisterResourceHandlerFactory.
func NewResourceHandler(opt *ResourceOptions) (ResourceHandler, error) {
	idx := strings.Index(opt.URL, "://")
	if idx < 0 {
		return nil, errors.NewNotValidf("[backend] Unknown scheme in URL: %q. Does not contain ://", opt.URL)
	}
	scheme := opt.URL[:idx]

	rh, err := newResourceHandlerFromFactory(scheme, opt)
	if err != nil {
		return nil, errors.Wrapf(err, "[backend] Failed to create new ResourceHandler object: %q", opt.URL)
	}
	return rh, nil
}

// Resource specifies the location to a 3rd party remote system within an Tag
// tag. A resource attribute (src="") can occur n-times.
type Resource struct {
	// Index specifies the number of occurrence within the include tag to
	// allowing sorting and hence having a priority list.
	Index int
	// URL to a remote 3rd party service which gets used by http.Client OR the
	// URL contains an alias to a connection to a KeyValue/gRPC server (defined
	// in the Caddyfile) to fetch a value via the field "Key" or "KeyTemplate".
	// This field gets copied to ResourceArgs.URL.
	url string

	// reqFunc performs a request to a backend service via a specific
	// protocol.
	handler ResourceHandler
	// circuit breaker http://martinfowler.com/bliki/CircuitBreaker.html
	cbFailures        uint64
	cbLastFailureTime uint64 //  in UnixNano
}

// MustNewResource same as NewResource but panics on error.
func MustNewResource(idx int, url string) *Resource {
	r, err := NewResource(idx, url)
	if err != nil {
		panic(err)
	}
	return r
}

// NewResource creates a new resource to one backend. Inspects the URL if it
// contains a template and parses that template.
func NewResource(idx int, url string) (*Resource, error) {
	r := &Resource{
		Index: idx,
		url:   url,
	}

	schemeAlias := r.url
	if pos := strings.Index(r.url, "://"); pos > 1 {
		schemeAlias = strings.ToLower(r.url[:pos])
	}

	var ok bool
	r.handler, ok = LookupResourceHandler(schemeAlias)
	if !ok {
		return nil, errors.NewNotSupportedf("[esibackend] NewResource protocol or alias %q not yet supported for URL/Alias %q", schemeAlias, r.url)
	}

	return r, nil
}

// String returns a resource identifier, most likely the underlying URL and the
// template name, if defined.
func (r *Resource) String() string {
	return r.url
}

// DoRequest performs the request to the backend resource. It generates the URL
// and then fires the request. DoRequest has the same signature as ResourceHandler
func (r *Resource) DoRequest(args *ResourceArgs) (http.Header, []byte, error) {

	args.URL = args.repl.Replace(r.url)
	args.Tag.Key = args.repl.Replace(args.Tag.Key)

	h, b, err := r.handler.DoRequest(args)
	if err != nil {
		err = errors.Wrap(err, "[esibackend] Resource.Handler.DoRequest")
	}
	return h, b, err
}

// CBState declares the different states for the circuit breaker (CB)
const (
	CBStateOpen = iota + 1
	CBStateHalfOpen
	CBStateClosed
)

// CBMaxFailures maximum amount of failures before the circuit breaker is half
// open to try the next request.
var CBMaxFailures uint64 = 12

// CBThresholdCalc calculates the threshold how long the CB should wait until to set the HalfOpen state.
// Default implementation returns an exponentially calculated duration
var CBThresholdCalc = func(failures uint64) time.Duration {
	return (1 << failures) * time.Second
}

// CBFailures number of failures. Thread safe.
func (r *Resource) CBFailures() uint64 {
	return atomic.LoadUint64(&r.cbFailures)
}

// CBState returns the current state of the circuit breaker and the last failure
// time. Thread safe.
func (r *Resource) CBState() (state int, lastFailure time.Time) {
	var thresholdPassed bool

	failures := atomic.LoadUint64(&r.cbFailures)
	lastFailed := int64(atomic.LoadUint64(&r.cbLastFailureTime))
	// increment the lastFailed with an exponential time out
	lastFailed += CBThresholdCalc(failures).Nanoseconds()

	secs := lastFailed / int64(time.Second)
	tn := time.Now()
	if secs > 0 {
		lastFailure = time.Unix(secs, lastFailed%secs)
		// only when the current time is in the future of the lastFailure then the
		// circuit breaker is half open.
		thresholdPassed = tn.After(lastFailure)
	}

	switch {
	case failures >= CBMaxFailures && thresholdPassed:
		state = CBStateHalfOpen
	case failures >= CBMaxFailures:
		state = CBStateOpen
	default:
		state = CBStateClosed
	}
	return state, lastFailure
}

// CBReset resets the circuit breaker. Thread safe.
func (r *Resource) CBReset() {
	atomic.StoreUint64(&r.cbLastFailureTime, 0)
	atomic.StoreUint64(&r.cbFailures, 0)
}

// CBRecordFailure records a failure and increases the internal counter. Returns
// the last failed time. Thread safe.
func (r *Resource) CBRecordFailure() (failedUnixNano int64) {
	atomic.AddUint64(&r.cbFailures, 1)
	// TODO(CyS) think about to switch to monotime.Now() for the whole CB
	failedUnixNano = time.Now().UnixNano()
	atomic.StoreUint64(&r.cbLastFailureTime, uint64(failedUnixNano))
	return failedUnixNano
}
