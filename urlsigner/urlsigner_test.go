package urlsigner

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSignAndParse(t *testing.T) {
	tcs := map[string]struct {
		p   Params
		err error
	}{
		"ok":      {p: Params{TeamID: "t1235", UserID: "u492skdjf", TZ: -2, ExpiryDuration: time.Hour}, err: nil},
		"expired": {p: Params{TeamID: "t1235", UserID: "u492skdjf", TZ: -2, ExpiryDuration: -1 * time.Second}, err: ErrExpiredSignature},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			e := New(randKey())

			sig := e.Sign(tc.p)
			require.NotEmpty(t, sig, "Signature should not be empty")
		})
	}
}

func TestParseEdges(t *testing.T) {
	e := New(randKey())
	valid := e.Sign(Params{Expiry: time.Now().Add(time.Hour).Unix()})

	tcs := map[string]string{
		"empty":            "",
		"empty json":       "7b7d",
		"wrong secret key": New(randKey()).Sign(Params{Expiry: time.Now().Add(time.Hour).Unix()}),
		"garbage prefix":   fmt.Sprintf("00%s", valid),
		"garbage suffix":   fmt.Sprintf("%s00", valid),
		"garbage":          "asdlkfjsldkjflskdjfl",
	}

	for name, in := range tcs {
		t.Run(name, func(t *testing.T) {

			_, err := e.Parse(in)
			require.Error(t, err, "")
		})
	}
}

func randKey() []byte {
	b := make([]byte, 256)
	_, err := rand.Read(b)
	if err != nil {
		panic(fmt.Sprintf("error reading random bytes: %v", err))
	}
	return b
}
