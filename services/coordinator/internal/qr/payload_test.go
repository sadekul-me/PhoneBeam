package qr

import (
	"testing"
	"time"
)

var trusted = []string{"http://127.0.0.1:8080", "http://10.0.2.2:8080"}

func TestParseValid(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	raw := `{"v":1,"origin":"http://127.0.0.1:8080","sid":"s","pid":"p","exp":1700000120}`
	got, err := Parse(raw, trusted, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.SID != "s" || got.PID != "p" {
		t.Fatalf("%+v", got)
	}
}

func TestParseMalformed(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cases := []string{
		"",
		"not-json",
		"https://evil.example/phish",
		"intent://scan",
		`{"v":1,"origin":"http://127.0.0.1:8080","sid":"s","exp":1700000120}`,
		`{"v":1,"origin":"http://127.0.0.1:8080","sid":"s","pid":"p","exp":1700000120,"token":"nope"}`,
	}
	for _, raw := range cases {
		if _, err := Parse(raw, trusted, now); err != ErrMalformed {
			t.Fatalf("%q err=%v", raw, err)
		}
	}
}

func TestParseUnknownVersion(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	raw := `{"v":99,"origin":"http://127.0.0.1:8080","sid":"s","pid":"p","exp":1700000120}`
	if _, err := Parse(raw, trusted, now); err != ErrUnknownVersion {
		t.Fatalf("err=%v", err)
	}
}

func TestParseUntrustedOrigin(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	raw := `{"v":1,"origin":"https://evil.example","sid":"s","pid":"p","exp":1700000120}`
	if _, err := Parse(raw, trusted, now); err != ErrUntrustedOrigin {
		t.Fatalf("err=%v", err)
	}
}

func TestParseExpired(t *testing.T) {
	now := time.Unix(1_700_000_120, 0)
	raw := `{"v":1,"origin":"http://127.0.0.1:8080","sid":"s","pid":"p","exp":1700000120}`
	if _, err := Parse(raw, trusted, now); err != ErrExpired {
		t.Fatalf("err=%v", err)
	}
}
