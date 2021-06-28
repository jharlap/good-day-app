package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNumberPrefixedEnumScan(t *testing.T) {
	tcs := []string{"", "a", "2", "5-ahe", "10-sd"}

	for _, tc := range tcs {
		t.Run(tc, func(t *testing.T) {
			var e NumberPrefixedEnum
			err := e.Scan(tc)

			require.NoError(t, err, "unexpected error")
			require.EqualValues(t, tc, e, "scanned value mismatch")
		})
	}
}

func TestNumberPrefixedEnumIntVal(t *testing.T) {
	tcs := []struct {
		in NumberPrefixedEnum
		ex int
	}{
		{NumberPrefixedEnum(""), -1},
		{NumberPrefixedEnum("a"), -1},
		{NumberPrefixedEnum("2"), 2},
		{NumberPrefixedEnum("5-ahe"), 5},
		{NumberPrefixedEnum("10-sd"), 1},
	}

	for _, tc := range tcs {
		t.Run(string(tc.in), func(t *testing.T) {
			require.Equal(t, tc.ex, tc.in.IntVal(), "int value mismatch")
		})
	}
}
