package helpers_test

import (
	"testing"

	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/stretchr/testify/assert"
)

func TestCommaListToSlice(t *testing.T) {
	assert.Exactly(t,
		[]string{"GET", "POST", "PATCH"},
		helpers.CommaListToSlice(`GET , POST, PATCH  `),
	)
	assert.Exactly(t,
		[]string{},
		helpers.CommaListToSlice(`   `),
	)
}
