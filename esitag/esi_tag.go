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
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/SchumacherFM/caddyesi/backend"
	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/dustin/go-humanize"
	"github.com/gavv/monotime"
	"golang.org/x/sync/errgroup"
)

// TemplateIdentifier if some strings contain these characters then a
// template.Template will be created. For now a resource key or an URL is
// supported.
const TemplateIdentifier = "{{"

// Entity represents a single fully parsed ESI tag
type Entity struct {
	Log     log.Logger
	RawTag  []byte
	DataTag DataTag

	// <pool> but kept here for easy testing, for now.
	MaxBodySize       uint64 // DefaultMaxBodySize 5MB
	Timeout           time.Duration
	ForwardHeaders    []string
	ForwardHeadersAll bool
	ReturnHeaders     []string
	ReturnHeadersAll  bool
	TTL               time.Duration
	// </pool>

	// OnError contains the content which gets injected into an erroneous ESI
	// tag when all reuqests are failing to its backends. If onError in the ESI
	// tag contains a file name, then that content gets loaded.
	OnError []byte
	// Key defines a key in a KeyValue server to fetch the value from.
	Key string
	// KeyTemplate gets created when the Key field contains the template
	// identifier. Then the Key field would be empty.
	KeyTemplate backend.TemplateExecer

	// Coalesce TODO(CyS) multiple external requests which triggers a backend
	// resource request gets merged into one backend request
	Coalesce bool

	// Race TODO(CyS) From the README: Add the attribute `race="true"` to fire
	// all resource requests at once and the one which is the fastest gets
	// served and the others dropped.
	Race bool

	// Resources contains multiple unique Resource entries, aka backend systems
	// likes redis instances or other micro services. Resources occur within one
	// single ESI tag. The resource attribute (src="") can occur multiple times.
	// The first item which successfully returns data gets its content used in
	// the response. If one item fails and we have multiple resources, the next
	// resource gets queried. All resources share the same scheme/protocol which
	// must handle the ResourceHandler.
	Resources []*backend.Resource // Any 3rd party servers

	// Conditioner TODO(CyS) depending on a condition an ESI tag gets executed or not.
	Conditioner

	// resourceRFAPool for the request arguments
	// https://twitter.com/_rsc/status/816710229861793795 ;-)
	resourceRFAPool sync.Pool
}

// SplitAttributes splits an ESI tag by its attributes. This function avoids regexp.
func SplitAttributes(raw string) ([]string, error) {
	// include src='https://micro.service/checkout/cart={{ .r "x"}}' timeout="9ms" onerror="nocart.html" forwardheaders="Cookie,Accept-Language,Authorization"

	var lastQuote rune
	f := func(c rune) bool {
		// I have no idea why my code is working ;-|
		switch {
		case c == lastQuote:
			lastQuote = 0
			return false
		case lastQuote != 0:
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c) || c == '='
		}
	}

	ret := strings.FieldsFunc(raw, f)
	if len(ret) == 0 {
		return []string{}, nil
	}

	ret = ret[1:] // first index is always the word "include", so drop it
	if len(ret)%2 == 1 {
		return nil, errors.NewNotValidf("[esitag] Imbalanced attributes in %#v", ret)
	}
	for i := 0; i < len(ret); i = i + 2 {
		val := ret[i+1]
		if l := len(val); l-1 > 1 {
			val = val[1 : len(val)-1] // drop first and last character, should be a quotation mark
		}
		ret[i+1] = strings.TrimSpace(val)
	}

	return ret, nil
}

// ParseRaw parses the RawTag field and fills the remaining fields of the
// struct.
func (et *Entity) ParseRaw() error {
	if len(et.RawTag) == 0 {
		return nil
	}
	et.Resources = make([]*backend.Resource, 0, 2)

	matches, err := SplitAttributes(string(et.RawTag))
	if err != nil {
		return errors.Wrap(err, "[esitag] Parse SplitAttributes")
	}

	srcCounter := 0
	for j := 0; j < len(matches); j = j + 2 {

		attr := matches[j]
		value := matches[j+1]

		switch attr {
		case "src":
			if err := et.parseResource(srcCounter, value); err != nil {
				return errors.Wrapf(err, "[caddyesi] Failed to parse src %q in tag %q", value, et.RawTag)
			}
			srcCounter++
		case "key":
			if err := et.parseKey(value); err != nil {
				return errors.Wrapf(err, "[caddyesi] Failed to parse src %q in tag %q", value, et.RawTag)
			}
		case "condition":
			if err := et.parseCondition(value); err != nil {
				return errors.Wrapf(err, "[caddyesi] Failed to parse condition %q in tag %q", value, et.RawTag)
			}
		case "onerror":
			if err := et.parseOnError(value); err != nil {
				return errors.Wrapf(err, "[caddyesi] Failed to parse onError %q in tag %q", value, et.RawTag)
			}
		case "timeout":
			var err error
			et.Timeout, err = time.ParseDuration(value)
			if err != nil {
				return errors.NewNotValidf("[caddyesi] ESITag.ParseRaw. Cannot parse duration in timeout: %s => %q\nTag: %q", err, value, et.RawTag)
			}
		case "ttl":
			var err error
			et.TTL, err = time.ParseDuration(value)
			if err != nil {
				return errors.NewNotValidf("[caddyesi] ESITag.ParseRaw. Cannot parse duration in ttl: %s => %q\nTag: %q", err, value, et.RawTag)
			}
		case "maxbodysize":
			var err error
			et.MaxBodySize, err = humanize.ParseBytes(value)
			if err != nil {
				return errors.NewNotValidf("[caddyesi] ESITag.ParseRaw. Cannot max body size in maxbodysize: %s => %q\nTag: %q", err, value, et.RawTag)
			}
		case "forwardheaders":
			if value == "all" {
				et.ForwardHeadersAll = true
			} else {
				et.ForwardHeaders = helpers.CommaListToSlice(value)
				for i, v := range et.ForwardHeaders {
					et.ForwardHeaders[i] = http.CanonicalHeaderKey(v)
				}
			}
		case "returnheaders":
			if value == "all" {
				et.ReturnHeadersAll = true
			} else {
				et.ReturnHeaders = helpers.CommaListToSlice(value)
				for i, v := range et.ReturnHeaders {
					et.ReturnHeaders[i] = http.CanonicalHeaderKey(v)
				}
			}
		default:
			// if an attribute starts with x we'll ignore it because the
			// developer might want to temporarily disable an attribute.
			if len(attr) > 1 && attr[0] != 'x' {
				return errors.NewNotSupportedf("[esitag] Unsupported attribute name %q with value %q", attr, value)
			}
		}
	}
	if len(et.Resources) == 0 || srcCounter == 0 {
		return errors.NewEmptyf("[caddyesi] ESITag.ParseRaw. src (Items: %d/Src: %d) cannot be empty in Tag which requires at least one resource: %q", len(et.Resources), srcCounter, et.RawTag)
	}

	et.InitPoolRFA(nil)

	return nil
}

func (et *Entity) parseOnError(val string) (err error) {
	var fileExt string
	if li := strings.LastIndexByte(val, '.'); li > 0 {
		fileExt = strings.ToLower(val[li+1:])
	}

	switch fileExt {
	case "html", "htm", "xml", "txt", "json":
		et.OnError, err = ioutil.ReadFile(filepath.Clean(val))
		if err != nil {
			return errors.NewFatalf("[caddyesi] ESITag.ParseRaw. Failed to process %q as template with error: %s\nTag: %q", val, err, et.RawTag)
		}
	default:
		et.OnError = []byte(val)
	}

	return nil
}

func (et *Entity) parseCondition(s string) error {
	tpl, err := backend.ParseTemplate("condition_tpl", s)
	if err != nil {
		return errors.NewFatalf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nTag: %q", s, err, et.RawTag)
	}
	et.Conditioner = condition{tpl: tpl}
	return nil
}

func (et *Entity) parseResource(idx int, val string) error {
	r, err := backend.NewResource(idx, val)
	if err != nil {
		return errors.Wrapf(err, "[caddyesi] ESITag.ParseRaw. Failed to parse %q\nTag: %q", val, et.RawTag)
	}
	et.Resources = append(et.Resources, r)
	return nil
}

func (et *Entity) parseKey(val string) (err error) {
	et.Key = val
	if strings.Contains(val, TemplateIdentifier) {
		et.KeyTemplate, err = backend.ParseTemplate("key_tpl", val)
		if err != nil {
			return errors.NewFatalf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nTag: %q", val, err, et.RawTag)
		}
	}
	return nil
}

// InitPoolRFA used in PathConfig.UpsertESITags and in Entity.ParseRaw to set
// the pool function. When called in PathConfig.UpsertESITags all default config
// values have been applied correctly.
func (et *Entity) InitPoolRFA(defaultRFA *backend.ResourceArgs) {

	if et.Log == nil && defaultRFA != nil {
		et.Log = defaultRFA.Log
	}
	if et.MaxBodySize == 0 && defaultRFA != nil {
		et.MaxBodySize = defaultRFA.MaxBodySize
	}
	if et.Timeout < 1 && defaultRFA != nil {
		et.Timeout = defaultRFA.Timeout
	}
	if et.TTL < 1 && defaultRFA != nil {
		et.TTL = defaultRFA.TTL
	}

	et.resourceRFAPool.New = func() interface{} {
		return &backend.ResourceArgs{
			Log:               et.Log,
			Key:               et.Key,
			KeyTemplate:       et.KeyTemplate,
			Timeout:           et.Timeout,
			MaxBodySize:       et.MaxBodySize,
			ForwardHeaders:    et.ForwardHeaders,
			ForwardHeadersAll: et.ForwardHeadersAll,
			ReturnHeaders:     et.ReturnHeaders,
			ReturnHeadersAll:  et.ReturnHeadersAll,
		}
	}
}

// used in Entity.QueryResources
func (et *Entity) poolGetRFA(externalReq *http.Request) *backend.ResourceArgs {
	rfa := et.resourceRFAPool.Get().(*backend.ResourceArgs)
	rfa.ExternalReq = externalReq
	return rfa
}

// used in Entity.QueryResources
func (et *Entity) poolPutRFA(rfa *backend.ResourceArgs) {
	rfa.ExternalReq = nil
	rfa.URL = ""
	et.resourceRFAPool.Put(rfa)
}

// QueryResources iterates sequentially over the resources and executes requests
// as defined in the ResourceHandler. If one resource fails it will be
// marked as timed out and the next resource gets tried. The exponential
// back-off stops when MaxBackOffs have been reached and then tries again.
// Returns a Temporary error behaviour when all requests to all resources are
// failing.
func (et *Entity) QueryResources(externalReq *http.Request) ([]byte, error) {
	timeStart := monotime.Now()

	var mErr *errors.MultiErr // just for collecting errors for informational purposes at the Temporary error at the end.
	rfa := et.poolGetRFA(externalReq)
	defer et.poolPutRFA(rfa)

	for i, r := range et.Resources {

		var lFields log.Fields
		if et.Log.IsDebug() {
			lFields = log.Fields{log.Int("resource_index", r.Index), log.String("url", r.String())}
		}

		switch state, lastFailure := r.CBState(); state {

		case backend.CBStateHalfOpen, backend.CBStateClosed:
			// TODO(CyS) add ReturnHeader
			_, data, err := r.DoRequest(rfa)
			if err != nil {
				mErr = mErr.AppendErrors(errors.Errorf("\nIndex %d URL %q with %s\n", i, r.String(), err))
				lastFailureTime := r.CBRecordFailure()
				if et.Log.IsDebug() {
					et.Log.Debug("esitag.Entity.QueryResources.ResourceHandler.Error",
						log.Duration(log.KeyNameDuration, monotime.Since(timeStart)),
						log.Err(err), log.Uint64("failure_count", r.CBFailures()), log.UnixNanoHuman("last_failure", lastFailureTime), lFields)
				}
				continue // go to next resource in this loop
			}
			if state == backend.CBStateHalfOpen {
				r.CBReset()
				if et.Log.IsDebug() {
					et.Log.Debug("esitag.Entity.QueryResources.ResourceHandler.CBStateHalfOpen",
						log.Duration(log.KeyNameDuration, monotime.Since(timeStart)),
						log.Uint64("failure_count", r.CBFailures()), log.Stringer("last_failure", lastFailure),
						lFields, log.String("content", string(data)))
				}
			} else if et.Log.IsDebug() {
				et.Log.Debug("esitag.Entity.QueryResources.ResourceHandler.CBStateClosed",
					log.Duration(log.KeyNameDuration, monotime.Since(timeStart)),
					log.Uint64("failure_count", r.CBFailures()), log.Stringer("last_failure", lastFailure),
					lFields, log.String("content", string(data)))
			}
			// TODO(CyS): Log header, create special function to log header; LOG rfa with special format
			return data, nil

		case backend.CBStateOpen:
			if et.Log.IsDebug() {
				et.Log.Debug("esitag.Entity.QueryResources.ResourceHandler.CBStateOpen",
					log.Duration(log.KeyNameDuration, monotime.Since(timeStart)),
					log.Uint64("failure_count", r.CBFailures()), log.Stringer("last_failure", lastFailure), lFields)
			}
		}

		// go to next resource
	}
	// error temporarily timeout so fall back to a maybe provided file.
	return nil, errors.NewTemporaryf("[esitag] Requests to all resources have temporarily failed: %s", mErr)
}

// Entities represents a list of ESI tags found in one HTML page.
type Entities []*Entity

// ApplyLogger sets a logger to each entity.
func (et Entities) ApplyLogger(l log.Logger) {
	for _, e := range et {
		e.Log = l
	}
}

// ParseRaw parses all ESI tags
func (et Entities) ParseRaw() error {
	for i := range et {
		if err := et[i].ParseRaw(); err != nil {
			return errors.Wrapf(err, "[caddyesi] Entities ParseRaw failed at index %d", i)
		}
	}
	return nil
}

// String for debugging only!
func (et Entities) String() string {
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	for i, e := range et {
		raw := e.RawTag
		e.RawTag = nil
		_, _ = fmt.Fprintf(buf, "%d: %#v\n", i, e)
		_, _ = fmt.Fprintf(buf, "%d: RawTag: %q\n\n", i, raw)
	}
	return buf.String()
}

// QueryResources runs in parallel to query all available backend services /
// resources which are available in the current page. The returned Tag slice is
// guaranteed to be sorted after Start position. If the request gets canceled
// via its context then all resource requests gets canceled too.
func (et Entities) QueryResources(r *http.Request) (DataTags, error) {

	if len(et) == 0 {
		return DataTags{}, nil
	}

	tags := make(DataTags, 0, len(et))
	g, ctx := errgroup.WithContext(r.Context())
	cTag := make(chan DataTag)
	for _, e := range et {
		e := e
		g.Go(func() error {
			data, err := e.QueryResources(r)
			isTempErr := errors.IsTemporary(err)
			if err != nil && !isTempErr {
				// err should have in most cases temporary error behaviour.
				// but here URL template rendering went wrong.
				return errors.Wrapf(err, "[esitag] QueryResources.Resources.DoRequest failed for Tag %q", e.RawTag)
			}

			t := e.DataTag
			t.Data = data
			if isTempErr {
				t.Data = e.OnError
			}

			select {
			case cTag <- t:
			case <-ctx.Done():
				return errors.Wrap(ctx.Err(), "[esitag] Context Done!")
			}
			return nil
		})
	}
	go func() {
		g.Wait()
		close(cTag)
	}()

	for t := range cTag {
		tags = append(tags, t)
	}

	// Check whether any of the goroutines failed. Since g is accumulating the
	// errors, we don't need to send them (or check for them) in the individual
	// results sent on the channel.
	if err := g.Wait(); err != nil {
		return nil, errors.Wrap(err, "[esitag] Entities.QueryResources ErrGroup.Error")
	}

	sort.Stable(tags)

	return tags, nil
}
