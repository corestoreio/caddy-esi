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

package esikv

import (
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"path/filepath"

	"github.com/corestoreio/errors"
)

// ConfigItem defines a single configuration item.
type ConfigItem struct {
	// Alias can have any name which gets used in an ESI tag and refers to the
	// connection to a resource.
	Alias string `xml:"alias" json:"alias"`
	// URL defines the authentication and target to a resource. If an URL
	// contains the name of an Alias then the URl data from that alias will be
	// copied into this URL field.
	URL string `xml:"url" json:"url"`
	// Query contains mostly a SQL query which runs as a prepared statement so you
	// must use the question mark or any other placeholder.
	Query string `xml:"query" json:"query"`
}

// NewConfigItem creates a new configuration
func NewConfigItem(url string, aliasQuery ...string) *ConfigItem {
	ci := &ConfigItem{
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

// ConfigItems as list of multiple configuration items. This type has internal
// helper functions.
type ConfigItems []*ConfigItem

func (ci ConfigItems) urlByAlias(alias string) string {
	for _, i := range ci {
		if i.Alias == alias && alias != "" {
			return i.URL
		}
	}
	return ""
}

// ConfigUnmarshal runs during Caddy setup and reads the extended resource
// configuration file. This file contains the alias name, the URL where the
// resource can be accessed and how and also in case of databases the query.
// Supported formats are for now XML and JSON. XML has the advantage of using
// CDATA and comments where in JSON you need to encode the strings properly. For
// security reasons you must store this fileName in a non-webserver accessible
// directory.
func ConfigUnmarshal(fileName string) (itms ConfigItems, _ error) {
	// TODO: as passwords gets stored in plain text in the JSON or XML file we
	// should create a feature where you pass the path to a PGP private key via
	// environment variable and then you can decode the file. alternatively
	// instead of read from a file we can query the data from etcd or consul. If
	// some wants to implement this feature send a PR.

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.NewFatalf("[esikv] Failed to read file %q with error: %s", fileName, err)
	}

	switch ext := filepath.Ext(fileName); ext {
	case ".json":
		if err := json.Unmarshal(data, &itms); err != nil {
			return nil, errors.NewFatalf("[esikv] Failed to parse JSON: %s", err)
		}
	case ".xml":
		var xi = &struct {
			XMLName xml.Name      `xml:"items"`
			Items   []*ConfigItem `xml:"item" json:"items"`
		}{}
		if err := xml.Unmarshal(data, xi); err != nil {
			return nil, errors.NewFatalf("[esikv] Failed to parse JSON: %s", err)
		}
		itms = ConfigItems(xi.Items)
	default:
		return nil, errors.NewNotSupportedf("[kvconfig] File extension %q not supported for filename %q", ext, fileName)
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
