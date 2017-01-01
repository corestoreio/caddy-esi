package helpers_test

import (
	"testing"

	"github.com/SchumacherFM/caddyesi/helpers"
	"github.com/stretchr/testify/assert"
)

func TestCommaListToSlice(t *testing.T) {
	t.Parallel()

	assert.Exactly(t,
		[]string{"GET", "POST", "PATCH"},
		helpers.CommaListToSlice(`GET , POST, PATCH  `),
	)
	assert.Exactly(t,
		[]string{},
		helpers.CommaListToSlice(`   `),
	)
}

func TestStringsToInts(t *testing.T) {
	assert.Exactly(t, []int{300, 400}, helpers.StringsToInts([]string{"300", "400"}))
	assert.Exactly(t, []int{300}, helpers.StringsToInts([]string{"300", "#"}))
	assert.Exactly(t, []int{}, helpers.StringsToInts([]string{"x", "y"}))
	assert.Exactly(t, []int{}, helpers.StringsToInts([]string{}))
}
