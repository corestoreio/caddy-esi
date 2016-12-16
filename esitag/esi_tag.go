package esitag

import (
	"bytes"
	"net/http"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"
)

// TemplateIdentifier if some strings contain these characters then a
// template.Template will be created. For now a resource key or an URL is
// supported.
const TemplateIdentifier = "{{"

// Resource specifies the location to a 3rd party remote system
type Resource struct {
	// Index specifies the number of occurrence within the include tag to allowing
	// sorting and hence having a priority list.
	Index int
	// URL location to make a network request
	URL string
	// KVNet defines a variable to a resource which has been defined once at the
	// top of the Caddyfile config. This prevents duplicated code to a resource.
	KVNet string
	// Template gets reated when the URL contains the template identifiers.
	Template *template.Template
}

// Resources contains multiple unique Resource entries, aka backend systems
// likes redis instances.
type Resources []Resource

// ResourceKey contains the key for a lookup in a 3rd party system, for example
// in Redis this key will be used to retrieve the value.
type ResourceKey struct {
	Key string
	// Template gets created when the Key contains the template identifiers.
	Template *template.Template
}

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

// Entity represents a single fully parsed ESI tag
type Entity struct {
	RawTag            []byte
	TagStart          int // start position in the stream
	TagEnd            int // end position in the stream
	Resources             // creates an URL to fetch data from
	ResourceKey           // use for lookup in key/value storages to fetch data from
	TTL               time.Duration
	Timeout           time.Duration
	OnError           string
	ForwardHeaders    []string
	ForwardHeadersAll bool
	ReturnHeaders     []string
	ReturnHeadersAll  bool
	Conditioner
}

var regexESITagDouble = regexp.MustCompile(`([a-z]+)="([^"\r\n]+)"|([a-z]+)='([^'\r\n]+)'`)

// ParseRaw parses the RawTag field and fills the remaining fields of the
// struct.
func (et *Entity) ParseRaw() error {
	if len(et.RawTag) == 0 {
		return nil
	}

	// it's kinda ridiculous because the ESI tag parser uses even sync.Pool to
	// reduce allocs and speed up processing and here we're relying on regex.
	// Usually those function for ESI tag parsing will only be called once and
	// then cached. we can optimize it later.
	matches := regexESITagDouble.FindAllSubmatch(et.RawTag, -1)

	srcCounter := 0
	for _, subs := range matches {
		if len(subs) != 5 {
			var bufSubs bytes.Buffer
			for _, s := range subs {
				bufSubs.Write(s)
				bufSubs.WriteRune('\n')
			}

			return errors.Errorf("[caddyesi] ESITag.ParseRaw: Incorrect number of regex matches: %q => All matches: %#v\nTag: %q", bufSubs, matches, et.RawTag)
		}
		// 1+2 defines the double quotes: key="product_234234"
		subsKey := subs[1]
		subsVal := subs[2]
		if len(subsKey) == 0 {
			// fall back to enclosed in single quotes: key='product_234234_{{ .r.Header.Get "myHeaderKey" }}'
			subsKey = subs[3]
			subsVal = subs[4]
		}
		key := string(subsKey)
		value := string(bytes.TrimSpace(subsVal))

		switch key {
		case "src":
			if err := et.parseResource(srcCounter, value); err != nil {
				return errors.Errorf("[caddyesi] Failed to parse src %q in tag %q with error:\n%+v", value, et.RawTag, err)
			}
			srcCounter++
		case "onerror":
			et.OnError = value
		case "key":
			if err := et.parseKey(value); err != nil {
				return errors.Errorf("[caddyesi] Failed to parse key %q in tag %q with error:\n%+v", value, et.RawTag, err)
			}
		case "condition":
			if err := et.parseCondition(value); err != nil {
				return errors.Errorf("[caddyesi] Failed to parse condition %q in tag %q with error:\n%+v", value, et.RawTag, err)
			}
		case "timeout":
			var err error
			et.Timeout, err = time.ParseDuration(value)
			if err != nil {
				return errors.Errorf("[caddyesi] ESITag.ParseRaw. Cannot parse duration in timeout: %s => %q\nTag: %q", err, value, et.RawTag)
			}
		case "ttl":
			var err error
			et.TTL, err = time.ParseDuration(value)
			if err != nil {
				return errors.Errorf("[caddyesi] ESITag.ParseRaw. Cannot parse duration in ttl: %s => %q\nTag: %q", err, value, et.RawTag)
			}
		case "forwardheaders":
			if value == "all" {
				et.ForwardHeadersAll = true
			} else {
				et.ForwardHeaders = commaListToSlice(value)
			}
		case "returnheaders":
			if value == "all" {
				et.ReturnHeadersAll = true
			} else {
				et.ReturnHeaders = commaListToSlice(value)
			}
			// default: ignore all other tags
		}
	}
	if len(et.Resources) == 0 {
		return errors.Errorf("[caddyesi] ESITag.ParseRaw. src cannot be empty in Tag which requires at least one resource: %q", et.RawTag)
	}
	return nil
}

func (et *Entity) parseKey(s string) error {
	if strings.Contains(s, TemplateIdentifier) {
		var err error
		et.ResourceKey.Template, err = template.New("key_tpl").Parse(s)
		if err != nil {
			return errors.Errorf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nTag: %q", s, err, et.RawTag)
		}
	} else {
		// Key can only be set when there is no template because we have to distinguish when to use the key and when to render one
		et.ResourceKey.Key = s
	}
	return nil
}

func (et *Entity) parseCondition(s string) error {
	tpl, err := template.New("condition").Parse(s)
	if err != nil {
		errors.Errorf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nTag: %q", s, err, et.RawTag)
	}
	et.Conditioner = condition{Template: tpl}
	return nil
}

func (et *Entity) parseResource(idx int, s string) error {
	var r Resource
	isURL := strings.Contains(s, "://")
	switch {
	case isURL && strings.Contains(s, TemplateIdentifier):
		tpl, err := template.New("resource_tpl").Parse(s)
		if err != nil {
			return errors.Errorf("[caddyesi] ESITag.ParseRaw. Failed to parse %q as template with error: %s\nTag: %q", s, err, et.RawTag)
		}
		r = Resource{Template: tpl}
	case isURL:
		r = Resource{URL: s}
	default:
		r = Resource{KVNet: s}
	}
	r.Index = idx
	et.Resources = append(et.Resources, r)
	return nil
}

// Entities represents a list of ESI tags
type Entities []*Entity

// ParseRaw all ESI tags
func (et Entities) ParseRaw() error {
	for i := range et {
		if err := et[i].ParseRaw(); err != nil {
			return errors.Wrapf(err, "[caddyesi] Entities ParseRaw failed at index %d", i)
		}
	}
	return nil
}
