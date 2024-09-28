package utils_test

import (
	"testing"
	"time"

	"github.com/localvar/xuandb/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestParseDuration(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		s        string
		negative bool
		result   time.Duration
	}{
		// positive cases
		{"0ns", false, 0},
		{"1ns", false, 1 * time.Nanosecond},
		{"2us", false, 2 * time.Microsecond},
		{"3µs", false, 3 * time.Microsecond},
		{"3μs", false, 3 * time.Microsecond},
		{"4ms", false, 4 * time.Millisecond},
		{"5s", false, 5 * time.Second},
		{"6m", false, 6 * time.Minute},
		{"7h", false, 7 * time.Hour},
		{"8d", false, 8 * 24 * time.Hour},
		{"9w", false, 9 * 7 * 24 * time.Hour},
		{"2w3d10h5m129s123ms456us789ns", false, (17*24+10)*time.Hour + 5*time.Minute + 129123456789*time.Nanosecond},
		{"9223372036854775807ns", true, 9223372036854775807 * time.Nanosecond},

		// invalid duration format
		{"0", true, 0},
		{"1n", true, 0},
		{"1nss", true, 0},
		{"2u", true, 0},
		{"3µ", true, 0},
		{"3μ", true, 0},
		{"4D", true, 0},
		{"d5", true, 0},
		{"-2w3d10h5m", true, 0},
		{"2w3d10H5m", true, 0},
		{"2ww3d10h5m", true, 0},
		{"2w3d10h5m11", true, 0},

		// overflow
		{"9223372036854775808ns", true, 0},
		{"9223372036854776us", true, 0},
		{"9223372036855ms", true, 0},
		{"9223372037s", true, 0},
		{"15251w", true, 0},
		{"15250w2d", true, 0},
	}

	t.Run("ParseDuration", func(t *testing.T) {
		for _, c := range cases {
			d, err := utils.ParseDuration(c.s)
			if c.negative {
				assert.NotNil(err)
			} else {
				assert.Nil(err)
				assert.Equal(c.result, d)
			}
		}
	})

	t.Run("MustParseDuration", func(t *testing.T) {
		for _, c := range cases {
			if c.negative {
				assert.Panics(func() {
					utils.MustParseDuration(c.s)
				})
			} else {
				assert.NotPanics(func() {
					d := utils.MustParseDuration(c.s)
					assert.Equal(c.result, d)
				})
			}
		}
	})
}

func TestFormatDuration(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		d      time.Duration
		result string
	}{
		{0, "0s"},
		{1, "1ns"},
		{20, "20ns"},
		{2 * time.Microsecond, "2us"},
		{1234 * time.Microsecond, "1ms234us"},
		{234 * time.Millisecond, "234ms"},
		{56 * time.Second, "56s"},
		{147 * time.Minute, "2h27m"},
		{88 * time.Hour, "3d16h"},
		{72 * time.Hour, "3d"},
		{336 * time.Hour, "2w"},
		{(168 + 5*24 + 1) * time.Hour, "1w5d1h"},
		{(168*100+5*24+1)*time.Hour + 23*time.Minute + 45*time.Second + 123456789*time.Nanosecond, "100w5d1h23m45s123ms456us789ns"},
	}

	for _, c := range cases {
		r := utils.FormatDuration(c.d)
		assert.Equal(c.result, r)
	}

	assert.Panics(func() {
		utils.FormatDuration(-1)
	})
}
