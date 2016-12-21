package esitag_test

import (
	"github.com/SchumacherFM/caddyesi"
	"github.com/SchumacherFM/caddyesi/esitag"
)

var _ caddyesi.ResourceFetcher = (*esitag.Resources)(nil)
