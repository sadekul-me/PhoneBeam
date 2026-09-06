package qr

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

const ProtocolVersion = 1

var (
	ErrMalformed       = errors.New("malformed pairing payload")
	ErrUnknownVersion  = errors.New("unknown protocol version")
	ErrUntrustedOrigin = errors.New("untrusted coordinator origin")
	ErrExpired         = errors.New("pairing payload expired")
)

type Payload struct {
	V      int    `json:"v"`
	Origin string `json:"origin"`
	SID    string `json:"sid"`
	PID    string `json:"pid"`
	Exp    int64  `json:"exp"`
}

func looksLikeURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		strings.HasPrefix(trimmed, "intent:") ||
		strings.HasPrefix(trimmed, "javascript:")
}

func Parse(raw string, trustedOrigins []string, now time.Time) (Payload, error) {
	if looksLikeURL(raw) {
		return Payload{}, ErrMalformed
	}
	if len(raw) == 0 || len(raw) > 4096 {
		return Payload{}, ErrMalformed
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var p Payload
	if err := dec.Decode(&p); err != nil {
		return Payload{}, ErrMalformed
	}
	if p.V != ProtocolVersion {
		return Payload{}, ErrUnknownVersion
	}
	if p.SID == "" || p.PID == "" || p.Origin == "" || p.Exp == 0 {
		return Payload{}, ErrMalformed
	}
	if !validOrigin(p.Origin) {
		return Payload{}, ErrUntrustedOrigin
	}
	if !originAllowed(p.Origin, trustedOrigins) {
		return Payload{}, ErrUntrustedOrigin
	}
	if now.Unix() >= p.Exp {
		return Payload{}, ErrExpired
	}
	return p, nil
}

func validOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	return true
}

func originAllowed(origin string, trusted []string) bool {
	want := strings.TrimRight(origin, "/")
	for _, item := range trusted {
		if strings.TrimRight(item, "/") == want {
			return true
		}
	}
	return false
}
