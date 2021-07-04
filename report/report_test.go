package report

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMondayOfWeekBeforeInUTC(t *testing.T) {
	tcs := []struct {
		time     string
		tzOffset int
		exp      string
	}{
		{"2021-07-02 14:25:01", 0, "2021-06-21 00:00:00"},
		{"2021-03-02 01:22:01", 0, "2021-02-22 00:00:00"},
		{"2020-03-02 01:22:01", 0, "2020-02-24 00:00:00"},
		{"2021-03-02 01:22:01", -4, "2021-02-22 04:00:00"},
		{"2021-03-02 01:22:01", 6, "2021-02-21 18:00:00"},
	}

	for i, tc := range tcs {
		t.Run(fmt.Sprintf("case %d %s %d", i, tc.time, tc.tzOffset), func(t *testing.T) {
			ref, err := time.Parse("2006-01-02 15:04:05", tc.time)
			require.NoError(t, err, "programmer error: reference test case time is invalid")

			res := mondayOfWeekBeforeInUTC(ref, tc.tzOffset)

			exp, err := time.Parse("2006-01-02 15:04:05", tc.exp)
			require.NoError(t, err, "programmer error: expected test case time is invalid")
			require.WithinDuration(t, exp, res, time.Nanosecond, "time should match")
		})
	}
}
