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
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/SchumacherFM/caddyesi/helper"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"github.com/dustin/go-humanize"
	"github.com/gavv/monotime"
	"github.com/pierrec/xxHash/xxHash64"
	"golang.org/x/sync/errgroup"
)

// Entity represents a single fully parsed Tag tag
type Entity struct {
	RawTag  []byte
	DataTag DataTag
	// OnError contains the content which gets injected into an erroneous Tag
	// tag when all reuqests are failing to its backends. If onError in the Tag
	// tag contains a file name, then that content gets loaded.
	OnError []byte
	Config
	// Race TODO(CyS) From the README: Add the attribute `race="true"` to fire
	// all resource requests at once and the one which is the fastest gets
	// served and the others dropped.
	Race bool
	// Resources contains multiple unique Resource entries, aka backend systems
	// likes redis instances or other micro services. Resources occur within one
	// single Tag tag. The resource attribute (src="") can occur multiple times.
	// The first item which successfully returns data gets its content used in
	// the response. If one item fails and we have multiple resources, the next
	// resource gets queried. All resources share the same scheme/protocol which
	// must handle the ResourceHandler.
	Resources []*Resource // Any 3rd party servers
	// Conditioner TODO(CyS) depending on a condition an Tag tag gets executed or not.
	Conditioner
}

// Config provides the configuration of a single Tag tag. This information gets
// passed on as an argument towards the backend resources and enriches the
// Entity type.
type Config struct {
	Log               log.Logger    // optional
	ForwardHeaders    []string      // optional, already treated with http.CanonicalHeaderKey
	ReturnHeaders     []string      // optional, already treated with http.CanonicalHeaderKey
	ForwardPostData   bool          // optional
	ForwardHeadersAll bool          // optional
	ReturnHeadersAll  bool          // optional
	Timeout           time.Duration // required
	// TTL retrieved content from a backend can live this time in the middleware
	// cache.
	TTL time.Duration // optional
	// MaxBodySize allowed max body size to read from the backend resource.
	MaxBodySize uint64 // required
	// Key also in type esitag.Entity
	Key string // optional (for KV Service)
	// Coalesce TODO(CyS) multiple external requests which triggers a backend
	// resource request gets merged into one backend request
	Coalesce bool
	// Above fields are special aligned to save space, see "aligncheck"
}

// SplitAttributes splits an Tag tag by its attributes. This function avoids regexp.
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
	et.Resources = make([]*Resource, 0, 2)

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
			et.Key = value
		case "coalesce":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return errors.NewNotValidf("[caddyesi] Failed to parse coalesce %q into bool value in tag %q with error %s", value, et.RawTag, err)
			}
			et.Coalesce = b
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
		case "forwardpostdata":
			value = strings.ToLower(value)
			et.ForwardPostData = value == "1" || value == "true"
		case "forwardheaders":
			if value == "all" {
				et.ForwardHeadersAll = true
			} else {
				et.ForwardHeaders = helper.CommaListToSlice(value)
				for i, v := range et.ForwardHeaders {
					et.ForwardHeaders[i] = http.CanonicalHeaderKey(v)
				}
			}
		case "returnheaders":
			if value == "all" {
				et.ReturnHeadersAll = true
			} else {
				et.ReturnHeaders = helper.CommaListToSlice(value)
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
	et.Conditioner = condition{}
	return nil
}

func (et *Entity) parseResource(idx int, val string) error {
	r, err := NewResource(idx, val)
	if err != nil {
		return errors.Wrapf(err, "[caddyesi] ESITag.ParseRaw. Failed to parse %q\nTag: %q", val, et.RawTag)
	}
	et.Resources = append(et.Resources, r)
	return nil
}

// SetDefaultConfig used in PathConfig.UpsertESITags and in Entity.ParseRaw to set
// the pool function. When called in PathConfig.UpsertESITags all default config
// values have been applied correctly.
func (et *Entity) SetDefaultConfig(tag Config) {

	if et.Config.Log == nil && tag.Log != nil {
		et.Config.Log = tag.Log
	}
	if et.Config.MaxBodySize == 0 && tag.MaxBodySize > 0 {
		et.Config.MaxBodySize = tag.MaxBodySize
	}
	if et.Config.Timeout < 1 && tag.Timeout > 0 {
		et.Config.Timeout = tag.Timeout
	}
	if et.Config.TTL < 1 && tag.TTL > 0 {
		et.Config.TTL = tag.TTL
	}
}

// QueryResources iterates sequentially over the resources and executes requests
// as defined in the ResourceHandler. If one resource fails it will be marked as
// timed out and the next resource gets tried. The exponential back-off stops
// when MaxBackOffs have been reached and then tries again. Returns a Temporary
// error behaviour when all requests to all resources have failed.
func (et *Entity) QueryResources(externalReq *http.Request) ([]byte, error) {
	timeStart := monotime.Now()

	// mErr: just for collecting errors for informational purposes at the
	// Temporary error at the end.
	var mErr *errors.MultiErr
	ra := NewResourceArgs(externalReq, "", et.Config)

	for i, r := range et.Resources {

		var lFields log.Fields
		if et.Log.IsDebug() {
			lFields = log.Fields{log.Int("resource_index", r.Index), log.String("resource_url", r.String()), log.Marshal("resource_arguments", ra)}
		}

		switch state, lastFailure := r.CBState(); state {

		case CBStateHalfOpen, CBStateClosed:
			// TODO(CyS) add ReturnHeader
			_, data, err := r.DoRequest(ra)

			if err != nil {

				if errors.IsNotFound(err) {
					if et.Log.IsDebug() {
						et.Log.Debug("esitag.Entity.QueryResources.ResourceHandler.NotFound",
							log.Err(err), log.Duration(log.KeyNameDuration, monotime.Since(timeStart)), lFields)
					}
					continue // go to next resource in this loop
				}

				// A real error and we must trigger the circuit breaker
				mErr = mErr.AppendErrors(errors.Errorf("\nIndex %d URL %q with %s\n", i, r.String(), err))
				lastFailureTime := r.CBRecordFailure()
				if et.Log.IsInfo() {
					et.Log.Info("esitag.Entity.QueryResources.ResourceHandler.Error",
						log.Duration(log.KeyNameDuration, monotime.Since(timeStart)),
						log.Err(err), log.Uint64("failure_count", r.CBFailures()), log.UnixNanoHuman("last_failure", lastFailureTime), lFields)
				}
				continue // go to next resource in this loop
			}

			if state == CBStateHalfOpen {
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
			// TODO(CyS): Log header, create special function to log header; LOG ra with special format
			return data, nil

		case CBStateOpen:
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

// Entities represents a list of Tag tags found in one HTML page.
type Entities []*Entity

// ApplyLogger sets a logger to each entity.
func (et Entities) ApplyLogger(l log.Logger) {
	for _, e := range et {
		e.Log = l
	}
}

// ParseRaw parses all Tag tags
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
		_, _ = fmt.Fprintf(buf, "%d: RawTag: %q\n", i, e.RawTag)
	}
	return buf.String()
}

// SplitCoalesce creates two new slices whose entries contain either coalesce or
// non coalesce ESI tags. Returns always non-nil slices.
func (et Entities) SplitCoalesce() (coalesce Entities, nonCoalesce Entities) {
	coalesce = make(Entities, 0, len(et))
	nonCoalesce = make(Entities, 0, len(et))
	for _, e := range et {
		if e.Coalesce {
			coalesce = append(coalesce, e)
		} else {
			nonCoalesce = append(nonCoalesce, e)
		}
	}
	return coalesce, nonCoalesce
}

// HasCoalesce returns true if there is at least one tag with enabled coalesce
// feature.
func (et Entities) HasCoalesce() bool {
	for _, e := range et {
		if e.Coalesce {
			return true
		}
	}
	return false
}

// UniqueID calculates a unique ID for all tags in the slice.
func (et Entities) UniqueID() uint64 {
	// can be put into a hash pool ;-)
	h := xxHash64.New(235711131719) // for now this seed will be used, found under the kitchen table.
	for _, e := range et {
		_, _ = h.Write(e.RawTag)
	}
	return h.Sum64()
}

// QueryResources runs in parallel to query all available backend services /
// resources which are available in the current page. The returned DataTags
// slice is guaranteed to be sorted after Start positions and non-nil. If the
// request gets canceled via its context then all resource requests gets
// cancelled too.
func (et Entities) QueryResources(r *http.Request) (DataTags, error) {

	if len(et) == 0 {
		return DataTags{}, nil
	}

	g, ctx := errgroup.WithContext(r.Context())
	cTag := make(chan DataTag)
	for _, e := range et {
		e := e
		g.Go(func() error {
			data, err := e.QueryResources(r)
			// A temporary error describes that we have problems reaching the
			// backend resource and that the circuit breaker has been triggered
			// or maybe even stopped querying.
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

	tags := make(DataTags, 0, len(et))
	for t := range cTag {
		tags = append(tags, t)
	}

	// Check whether any of the goroutines failed. Since g is accumulating the
	// errors, we don't need to send them (or check for them) in the individual
	// results sent on the channel.
	if err := g.Wait(); err != nil {
		return DataTags{}, errors.Wrap(err, "[esitag] Entities.QueryResources ErrGroup.Error")
	}

	// restore original order as the tags occur on the HTML page
	sort.Sort(tags)

	return tags, nil
}
