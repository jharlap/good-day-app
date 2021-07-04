package urlsigner

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Params struct {
	TeamID string `json:"t"`
	UserID string `json:"u"`
	TZ     int    `json:"z"`
	Expiry int64  `json:"ts"`
	HMAC   []byte `json:"h"`

	ExpiryDuration time.Duration `json:"-"`
}

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrExpiredSignature = errors.New("expired signature")
)

type Engine struct {
	key []byte
}

func New(key []byte) *Engine {
	return &Engine{key: key}
}

func (e *Engine) Sign(p Params) string {
	if p.Expiry == 0 && p.ExpiryDuration > 0 {
		p.Expiry = time.Now().Add(p.ExpiryDuration).Unix()
	}
	p.HMAC = e.hmac(p)

	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}

	return hex.EncodeToString(b)
}

func (e *Engine) Parse(sig string) (Params, error) {
	b, err := hex.DecodeString(sig)
	if err != nil {
		return Params{}, fmt.Errorf("error decoding string: %w", err)
	}

	var p Params
	err = json.Unmarshal(b, &p)
	if err != nil {
		return Params{}, fmt.Errorf("error unmarshaling json: %w", err)
	}

	hm := e.hmac(p)
	if !hmac.Equal(hm, p.HMAC) {
		return Params{}, ErrInvalidSignature
	}

	if p.Expiry < time.Now().Unix() {
		return Params{}, ErrExpiredSignature
	}

	return p, nil
}

func (e *Engine) hmac(p Params) []byte {
	mac := hmac.New(sha1.New, e.key)
	mac.Write([]byte(fmt.Sprintf("%s:%s:%d:%d", p.TeamID, p.UserID, p.TZ, p.Expiry)))
	return mac.Sum(nil)
}
