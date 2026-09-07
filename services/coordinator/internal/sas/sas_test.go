package sas

import "testing"

func TestMediaDisplayBindsFingerprints(t *testing.T) {
	key := []byte("test-key-not-for-production")
	pairing := Display(key, "http://127.0.0.1:8080", "sid", "pid", "Op", []string{"screen.read"})
	a := MediaDisplay(key, "http://127.0.0.1:8080", "sid", "pid", "Op", []string{"screen.read"},
		"AB:CD:EF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:01:23:45:67:89:AB:CD:EF:00:11:22:33:44",
		"00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff:01:23:45:67:89:ab:cd:ef:00:11:22:33:44:55:66:77",
	)
	b := MediaDisplay(key, "http://127.0.0.1:8080", "sid", "pid", "Op", []string{"screen.read"},
		"abcdef00112233445566778899aabbccddeeff0123456789abcdef0011223344",
		"00112233445566778899aabbccddeeff0123456789abcdef0011223344556677",
	)
	if a != b {
		t.Fatalf("canonicalization failed %q vs %q", a, b)
	}
	if a == pairing {
		t.Fatal("media SAS must differ from pairing SAS")
	}
	swapped := MediaDisplay(key, "http://127.0.0.1:8080", "sid", "pid", "Op", []string{"screen.read"},
		"00112233445566778899aabbccddeeff0123456789abcdef0011223344556677",
		"abcdef00112233445566778899aabbccddeeff0123456789abcdef0011223344",
	)
	if swapped == a {
		t.Fatal("role-labeled fingerprints must not be commutative")
	}
}

func TestGoldenMediaSASVector(t *testing.T) {
	key := []byte("phonebeam-sas-test-key-32bytes!!")
	got := MediaDisplay(
		key,
		"http://127.0.0.1:8080",
		"sid-a",
		"pid-b",
		"Support-A",
		[]string{"input.control", "screen.read"},
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
	)
	if got != "830 034" {
		t.Fatalf("golden vector drifted: %q", got)
	}
}

func TestValidFingerprint(t *testing.T) {
	if !ValidSHA256Fingerprint("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") {
		t.Fatal("expected valid")
	}
	if ValidSHA256Fingerprint("deadbeef") {
		t.Fatal("too short")
	}
}
