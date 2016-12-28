package esitag

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/SchumacherFM/caddyesi/bufpool"
	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
	"golang.org/x/sync/errgroup"
)

// TemplateIdentifier if some strings contain these characters then a
// template.Template will be created. For now a resource key or an URL is
// supported.
const TemplateIdentifier = "{{"

// Conditioner does not represent your favorite shampoo but it gives you the
// possibility to define an expression which gets executed for every request to
// include the ESI resource or not.
type Conditioner interface {
	OK(r *http.Request) bool
}

type condition struct {
	*template.Template
}

func (c condition) OK(r *http.Request) bool {
	// todo
	return false
}

// DataTag identifies an ESI tag by its start and end position in the HTML byte
// stream for replacing. If the HTML changes there needs to be a refresh call to
// re-parse the HTML.
type DataTag struct {
	// Data from the micro service gathered in a goroutine.
	Data  []byte
	Start int // start position in the stream
	End   int // end position in the stream
}

// DataTags a list of tags with their position within a page and the content
type DataTags []DataTag

// InjectContent reads from r and uses the data in a Tag to get injected a the
// current position and then writes the output to w. DataTags must be a sorted
// slice. Usually this function receives the data from Entities.QueryResources()
func (dts DataTags) InjectContent(r io.Reader, w io.Writer) error {
	if len(dts) == 0 {
		return nil
	}

	dataBuf := bufpool.Get()
	defer bufpool.Put(dataBuf)
	data := dataBuf.Bytes()

	var prevBufDataSize int
	for di, dt := range dts {
		bufDataSize := dt.End - prevBufDataSize

		if cap(data) < bufDataSize {
			dataBuf.Grow(bufDataSize - cap(data))
			data = dataBuf.Bytes()
		}
		data = data[:bufDataSize]

		n, err := r.Read(data)
		if err != nil && err != io.EOF {
			return errors.NewFatalf("[esitag] Read failed: %s for tag index %d with start position %d and end position %d", err, di, dt.Start, dt.End)
		}

		if n > 0 {
			esiStartPos := n - (dt.End - dt.Start)
			if _, errW := w.Write(data[:esiStartPos]); errW != nil { // cuts off until End
				return errors.NewWriteFailedf("[esitag] Failed to write page data to w: %s", errW)
			}
			if _, errW := w.Write(dt.Data); errW != nil {
				return errors.NewWriteFailedf("[esitag] Failed to write ESI data to w: %s", errW)
			}
		}
		prevBufDataSize = dt.End
	}

	// during copy of the remaining bytes we'll hit EOF
	if _, err := io.Copy(w, r); err != nil {
		return errors.NewWriteFailedf("[esitag] Failed to copy remaining data to w: %s", err)
	}

	return nil
}

func (dts DataTags) Len() int           { return len(dts) }
func (dts DataTags) Swap(i, j int)      { dts[i], dts[j] = dts[j], dts[i] }
func (dts DataTags) Less(i, j int) bool { return dts[i].Start < dts[j].Start }

// Entity represents a single fully parsed ESI tag
type Entity struct {
	Log               log.Logger
	RawTag            []byte
	DataTag           DataTag
	MaxBodySize       int64 // DefaultMaxBodySize; TODO(CyS) implement in ESI tag
	TTL               time.Duration
	Timeout           time.Duration
	OnError           string
	ForwardHeaders    []string
	ForwardHeadersAll bool
	ReturnHeaders     []string
	ReturnHeadersAll  bool
	// Key defines a key in a KeyValue server to fetch the value from.
	Key string
	// KeyTemplate gets created when the Key field contains the template
	// identifier. Then the Key field would be empty.
	KeyTemplate *template.Template

	// ResourceRequestFunc performs a request to a backend service via a specific
	// protocol.
	ResourceRequestFunc
	// Resources contains multiple unique Resource entries, aka backend systems
	// likes redis instances or other micro services. Resources occur within one
	// single ESI tag. The resource attribute (src="") can occur multiple times.
	// The first item which successfully returns data gets its content used in
	// the response. If one item fails and we have multiple resources, the next
	// resource gets queried. All resources share the same scheme/protocol which
	// must handle the ResourceRequestFunc.
	Resources []*Resource // Any 3rd party servers

	Conditioner // todo
}

// SplitAttributes splits an ESI tag by its attributes
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
			if err := et.parseKey(value); err != nil {
				return errors.Wrapf(err, "[caddyesi] Failed to parse src %q in tag %q", value, et.RawTag)
			}
		case "condition":
			if err := et.parseCondition(value); err != nil {
				return errors.Wrapf(err, "[caddyesi] Failed to parse condition %q in tag %q", value, et.RawTag)
			}
		case "onerror":
			et.OnError = value
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
		case "forwardheaders":
			if value == "all" {
				et.ForwardHeadersAll = true
			} else {
				et.ForwardHeaders = helpers.CommaListToSlice(value)
			}
		case "returnheaders":
			if value == "all" {
				et.ReturnHeadersAll = true
			} else {
				et.ReturnHeaders = helpers.CommaListToSlice(value)
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
	if err := et.setResourceRequestFunc(); err != nil {
		return errors.Wrap(err, "[caddyesi] Entity.ParseRaw.setResourceRequestFunc failed")
	}
	return nil
}

func (et *Entity) setResourceRequestFunc() error {
	if len(et.Resources) == 0 {
		return nil
	}

	// we only check for the first index URL because sub sequent entries must
	// use the same protocol to fall back resources.

	idx := strings.Index(et.Resources[0].URL, "://")
	if idx == -1 {
		// do nothing because the string points to an alias to a globally
		// defined URL in the Caddyfile.
		return nil
	}

	scheme := strings.ToLower(et.Resources[0].URL[:idx])
	var ok bool
	et.ResourceRequestFunc, ok = ResourceRequestRegister[scheme]
	if !ok {
		return errors.NewNotSupportedf("[esitag] Resource protocal %q not yet supported in tag %q", scheme, et.RawTag)
	}
	return nil
}

func (et *Entity) parseCondition(s string) error {
	tpl, err := template.New("condition_tpl").Parse(s)
	if err != nil {
		return errors.NewFatalf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nTag: %q", s, err, et.RawTag)
	}
	et.Conditioner = condition{Template: tpl}
	return nil
}

func (et *Entity) parseResource(idx int, val string) (err error) {
	r := &Resource{
		Index: idx,
		IsURL: strings.Contains(val, "://"),
		URL:   val,
	}
	if r.IsURL && strings.Contains(val, TemplateIdentifier) {
		r.URLTemplate, err = template.New("resource_tpl").Parse(val)
		if err != nil {
			return errors.NewFatalf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nTag: %q", val, err, et.RawTag)
		}
	}
	et.Resources = append(et.Resources, r)
	return nil
}

func (et *Entity) parseKey(val string) (err error) {
	et.Key = val
	if strings.Contains(val, TemplateIdentifier) {
		et.KeyTemplate, err = template.New("key_tpl").Parse(val)
		if err != nil {
			return errors.NewFatalf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nTag: %q", val, err, et.RawTag)
		}
		et.Key = "" // unset Key because we have a template
	}
	return nil
}

// QueryResources iterates sequentially over the resources and executes requests
// as defined in the ResourceRequestFunc. If one resource fails it will be
// marked as timed out and the next resource gets tried. The exponential
// back-off stops when MaxBackOffs have been reached and then tries again.
func (et *Entity) QueryResources(externalReq *http.Request) ([]byte, error) {

	maxBodySize := DefaultMaxBodySize
	if et.MaxBodySize > 0 {
		maxBodySize = et.MaxBodySize
	}

	for i, r := range et.Resources {

		url := r.URL
		if r.URLTemplate != nil {
			buf := bufpool.Get()
			if err := r.URLTemplate.Execute(buf, externalReq); err != nil {
				bufpool.Put(buf)
				return nil, errors.Wrapf(err, "[esitag] Index %d Resource %#v Template error", i, r)
			}
			url = buf.String()
			bufpool.Put(buf)
		}

		var lFields log.Fields
		now := time.Now()
		if et.Log.IsDebug() {
			lFields = log.Fields{
				log.Int("bacled_off", r.backedOff), log.Int("index", i), log.Int("resources_length", len(et.Resources)),
				log.String("url", url), log.Time("last_failed", r.lastFailed), log.Time("now", now),
			}
		}

		if r.lastFailed.After(now) {
			if et.Log.IsDebug() {
				et.Log.Debug("esitag.Resources.DoRequest.lastFailed.After", lFields...)
			}
			if r.backedOff >= MaxBackOffs {
				if et.Log.IsDebug() {
					et.Log.Debug("esitag.Resources.DoRequest.backedOff", lFields...)
				}
				r.lastFailed = time.Time{} // restart
			}
			continue
		}

		data, err := et.ResourceRequestFunc(url, et.Timeout, maxBodySize)
		if err != nil {
			if et.Log.IsDebug() {
				et.Log.Debug("esitag.Resources.DoRequest.ResourceRequestFunc", log.Err(err), lFields)
			}
			r.backOff++
			var dur time.Duration = 1 << r.backOff // exponentially calculated
			r.lastFailed = time.Now().Add(dur * time.Second)
			continue
		}

		return data, nil
	}
	// error temporarily timeout so fall back to a maybe provided file.
	return nil, errors.Errorf("[esitag] Should maybe not happen? TODO investigate")
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

	tags := make(DataTags, 0, len(et))
	g, ctx := errgroup.WithContext(r.Context())
	cTag := make(chan DataTag)
	for _, e := range et {
		e := e
		g.Go(func() error {
			data, err := e.QueryResources(r)
			if err != nil {
				return errors.Wrapf(err, "[esitag] QueryResources.Resources.DoRequest failed for Tag %q", e.RawTag)
			}
			t := e.DataTag
			t.Data = data

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
		return nil, errors.Wrap(err, "[esitag]")
	}

	sort.Stable(tags)

	return tags, nil
}
