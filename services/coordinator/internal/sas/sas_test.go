package sas

import "testing"

func TestDisplayStableAndGrouped(t *testing.T) {
	key := []byte("test-key-not-for-production")
	a := Display(key, "http://127.0.0.1:8080", "sid", "pid", "Op", []string{"input.control", "screen.read"})
	b := Display(key, "http://127.0.0.1:8080", "sid", "pid", "Op", []string{"screen.read", "input.control"})
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
	if len(a) != 7 || a[3] != ' ' {
		t.Fatalf("unexpected format %q", a)
	}
	other := Display(key, "http://evil.example", "sid", "pid", "Op", []string{"screen.read", "input.control"})
	if other == a {
		t.Fatal("origin must bind sas")
	}
}
