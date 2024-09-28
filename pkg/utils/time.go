package utils

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidDuration is returned when the duration string is invalid.
	ErrInvalidDuration = errors.New("invalid duration")
	// ErrDurationOverflow is returned when the duration value overflows
	// time.Duration, i.e. int64.
	ErrDurationOverflow = errors.New("duration overflow")
)

// ParseDuration parses a duration string. We need this function because
// 'time.ParseDuration' does not support 'd'(day) and 'w'(week) units. However
// 'time.ParseDuration' supports negative duration and floating point numbers,
// which this function does not support.
func ParseDuration(s string) (time.Duration, error) {
	var d uint64

	for i := 0; i < len(s); {
		// search for digits
		start, v := i, uint64(0)
		for i < len(s) {
			c := uint64(s[i]) - '0'
			if c > 9 { // s[i] < '0' is also result in c > 9
				break
			}
			if (1<<63-c)/10 <= v {
				return 0, ErrDurationOverflow
			}
			v = v*10 + c
			i++
		}
		if i == start {
			return 0, ErrInvalidDuration
		}

		// search for unit
		start, unit := i, uint64(0)
		for i < len(s) && (s[i] < '0' || s[i] > '9') {
			i++
		}

		switch s[start:i] {
		case "ns":
			unit = uint64(time.Nanosecond)
		case "us", "µs", "μs":
			unit = uint64(time.Microsecond)
		case "ms":
			unit = uint64(time.Millisecond)
		case "s":
			unit = uint64(time.Second)
		case "m":
			unit = uint64(time.Minute)
		case "h":
			unit = uint64(time.Hour)
		case "d":
			unit = uint64(24 * time.Hour)
		case "w":
			unit = uint64(7 * 24 * time.Hour)
		default:
			return 0, ErrInvalidDuration
		}

		if 1<<63/unit <= v {
			return 0, ErrDurationOverflow
		}

		// add to duration
		v *= unit
		if 1<<63-v <= d {
			return 0, ErrDurationOverflow
		}
		d += v
	}

	return time.Duration(d), nil
}

// MustParseDuration parses a duration string as ParseDuration, but panics
// if the input string is invalid or overflow.
func MustParseDuration(s string) time.Duration {
	d, err := ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

// FormatDuration formats a duration to a string which can be parsed by
// ParseDuration, it panics if the duration is negative.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		panic("negative duration")
	}

	if d == 0 {
		return "0s"
	}

	sb := strings.Builder{}
	sb.Grow(20)

	if v := d / (7 * 24 * time.Hour); v > 0 {
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
		sb.WriteByte('w')
		d -= v * 7 * 24 * time.Hour
	}

	if v := d / (24 * time.Hour); v > 0 {
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
		sb.WriteByte('d')
		d -= v * 24 * time.Hour
	}

	if v := d / time.Hour; v > 0 {
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
		sb.WriteByte('h')
		d -= v * time.Hour
	}

	if v := d / time.Minute; v > 0 {
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
		sb.WriteByte('m')
		d -= v * time.Minute
	}

	if v := d / time.Second; v > 0 {
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
		sb.WriteByte('s')
		d -= v * time.Second
	}

	if v := d / time.Millisecond; v > 0 {
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
		sb.WriteString("ms")
		d -= v * time.Millisecond
	}

	if v := d / time.Microsecond; v > 0 {
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
		sb.WriteString("us")
		d -= v * time.Microsecond
	}

	if d > 0 {
		sb.WriteString(strconv.FormatUint(uint64(d), 10))
		sb.WriteString("ns")
	}

	return sb.String()
}
