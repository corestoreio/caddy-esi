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

package caddyesi

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/corestoreio/errors"
)

// ResourceItem defines a single configuration item.
type ResourceItem struct {
	// Alias can have any name which gets used in an ESI tag and refers to the
	// connection to a resource.
	Alias string `xml:"alias" json:"alias"`
	// URL defines the authentication and target to a resource. If an URL
	// contains the name of an Alias then the URl data from that alias will be
	// copied into this URL field.
	URL string `xml:"url" json:"url"`
	// Query contains mostly a SQL query which runs as a prepared statement so you
	// must use the question mark or any other placeholder.
	Query string `xml:"query,omitempty" json:"query"`
}

// NewResourceItem creates a new resource item. Supports up to 3 arguments.
func NewResourceItem(url string, aliasQuery ...string) *ResourceItem {
	ci := &ResourceItem{
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

// ResourceItems as list of multiple configuration items. This type has internal
// helper functions.
type ResourceItems []*ResourceItem

// WriteTo writes the XML into w and may return an error. It returns always zero
// bytes written :-(.
func (ci ResourceItems) WriteTo(w io.Writer) (n int64, err error) {
	return 0, errors.Wrap(ci.toXML(w), "[backend] ResourceItems.WriteTo failed")
}

func (ci ResourceItems) urlByAlias(alias string) string {
	for _, i := range ci {
		if i.Alias == alias && alias != "" {
			return i.URL
		}
	}
	return ""
}

// MustToXML transforms the object into a strings and panics on error. Only used
// in testing.
func (ci ResourceItems) toXML(w io.Writer) error {
	var xi = &struct {
		XMLName xml.Name        `xml:"items"`
		Items   []*ResourceItem `xml:"item" json:"items"`
	}{
		Items: ci,
	}

	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`))
	return errors.Wrap(xml.NewEncoder(w).Encode(xi), "[backend] ResourceItems.toXML failed")
}

// MustToXML transforms the object into a strings and panics on error. Only used
// in testing.
func (ci ResourceItems) MustToXML() string {
	var b bytes.Buffer
	if err := ci.toXML(&b); err != nil {
		panic(err)
	}
	return b.String()
}

// UnmarshalResourceItems runs during Caddy setup and reads the extended resource
// configuration file. This file contains the alias name, the URL where the
// resource can be accessed and how and also in case of databases the query.
// Supported formats are for now XML and JSON. XML has the advantage of using
// CDATA and comments where in JSON you need to encode the strings properly. For
// security reasons you must store this fileName in a non-webserver accessible
// directory.
func UnmarshalResourceItems(fileName string) (itms ResourceItems, err error) {
	// for test purposes the fileName can also contain the real content.

	// TODO: as passwords gets stored in plain text in the JSON or XML file we
	// should create a feature where you pass the path to a PGP private key via
	// environment variable and then you can decode the file. alternatively
	// instead of read from a file we can query the data from etcd or consul. If
	// some wants to implement this feature send a PR.

	const (
		shouldReadFile = iota + 1
		typeJSON
		typeXML
	)

	var data []byte
	var extType int
	switch ext := filepath.Ext(fileName); ext {
	case ".json":
		extType = setBit(extType, typeJSON)
		extType = setBit(extType, shouldReadFile)
	case ".xml":
		extType = setBit(extType, typeXML)
		extType = setBit(extType, shouldReadFile)
	default:
		switch {
		case strings.HasPrefix(fileName, "["):
			extType = setBit(extType, typeJSON)
		case strings.HasPrefix(fileName, "<?xml"):
			extType = setBit(extType, typeXML)
		default:
			return nil, errors.NewNotSupportedf("[kvconfig] Content-Type %q not supported", fileName)
		}
	}

	if hasBit(extType, shouldReadFile) {
		data, err = ioutil.ReadFile(fileName)
		if err != nil {
			return nil, errors.NewFatalf("[backend] Failed to read file %q with error: %s", fileName, err)
		}
	} else {
		data = []byte(fileName)
	}

	switch {
	case hasBit(extType, typeJSON):
		if err := json.Unmarshal(data, &itms); err != nil {
			return nil, errors.NewFatalf("[backend] Failed to parse JSON: %s", err)
		}
	case hasBit(extType, typeXML):
		var xi = &struct {
			XMLName xml.Name        `xml:"items"`
			Items   []*ResourceItem `xml:"item" json:"items"`
		}{}
		if err := xml.Unmarshal(data, xi); err != nil {
			return nil, errors.NewFatalf("[backend] Failed to parse XML: %s\n%s", err, string(data))
		}
		itms = ResourceItems(xi.Items)
	default:
		return nil, errors.NewNotSupportedf("[kvconfig] Content-Type %q not supported", fileName)
	}

	// If the field URL contains the name of an Alias of any other field, then copy
	// the URL data into the current aliased URL field to overwrite the alias
	for idx, itm := range itms {
		if u := itms.urlByAlias(itm.URL); u != "" {
			itms[idx].URL = u
		}
	}
	return itms, nil
}
func setBit(n int, pos uint) int {
	n |= (1 << pos)
	return n
}
func hasBit(n int, pos uint) bool {
	val := n & (1 << pos)
	return (val > 0)
}
