package protocol

import (
	"testing"
)

func TestParseRejectsUnknownVersionAndJunk(t *testing.T) {
	if _, err := Parse([]byte(`{"v":9,"type":"hello","token":"x"}`)); err != ErrWrongVersion {
		t.Fatalf("err=%v", err)
	}
	if _, err := Parse([]byte(`not-json`)); err != ErrMalformed {
		t.Fatalf("err=%v", err)
	}
}

func TestOfferBeforeApprovalRejected(t *testing.T) {
	env, err := Parse([]byte(`{"v":1,"type":"sdp_offer","sdp":{"type":"offer","sdp":"v=0\r\no=- 1 1 IN IP4 0.0.0.0\r\n"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(env, "phone", "CAPS_BOUND", true); err != ErrNotAllowed {
		t.Fatalf("err=%v", err)
	}
}

func TestOperatorCannotSendOffer(t *testing.T) {
	env, _ := Parse([]byte(`{"v":1,"type":"sdp_offer","sdp":{"type":"offer","sdp":"v=0\r\n"}}`))
	if err := Validate(env, "operator", "NEGOTIATING", true); err != ErrNotAllowed {
		t.Fatalf("err=%v", err)
	}
}

func TestMalformedICERejected(t *testing.T) {
	env, _ := Parse([]byte(`{"v":1,"type":"ice_candidate","candidate":{"candidate":"turn:evil","sdp_mid":"0","sdp_mline_index":0}}`))
	if err := Validate(env, "phone", "NEGOTIATING", true); err != ErrInvalidICE {
		t.Fatalf("err=%v", err)
	}
}

func TestFingerprintFormat(t *testing.T) {
	env, _ := Parse([]byte(`{"v":1,"type":"peer_fingerprint","fingerprint":{"algorithm":"sha-256","value":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","role":"phone"}}`))
	if err := Validate(env, "phone", "NEGOTIATING", true); err != nil {
		t.Fatal(err)
	}
	bad, _ := Parse([]byte(`{"v":1,"type":"peer_fingerprint","fingerprint":{"algorithm":"md5","value":"abcd","role":"phone"}}`))
	if err := Validate(bad, "phone", "NEGOTIATING", true); err != ErrInvalidFingerprint {
		t.Fatalf("err=%v", err)
	}
}

func TestExtractFingerprint(t *testing.T) {
	sdp := "v=0\r\na=fingerprint:sha-256 AB:CD:EF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:01:23:45:67:89:AB:CD:EF:00:11:22:33:44\r\n"
	fp, ok := ExtractFingerprint(sdp)
	if !ok || fp != "abcdef00112233445566778899aabbccddeeff0123456789abcdef0011223344" {
		t.Fatalf("fp=%s ok=%v", fp, ok)
	}
}

func TestParseRejectsOversizedAndMissingSDP(t *testing.T) {
	huge := make([]byte, MaxMessageBytes+1)
	if _, err := Parse(huge); err != ErrMalformed {
		t.Fatalf("err=%v", err)
	}
	env, _ := Parse([]byte(`{"v":1,"type":"sdp_answer","sdp":{"type":"answer","sdp":""}}`))
	if err := Validate(env, "operator", "NEGOTIATING", true); err != ErrInvalidSDP {
		t.Fatalf("err=%v", err)
	}
}

func TestHangupAllowedBeforeNegotiate(t *testing.T) {
	env, err := Parse([]byte(`{"v":1,"type":"hangup"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(env, "operator", "CAPS_BOUND", true); err != nil {
		t.Fatal(err)
	}
}

func TestPeerReadyRejectedBeforeProjection(t *testing.T) {
	env, _ := Parse([]byte(`{"v":1,"type":"peer_ready"}`))
	if err := Validate(env, "phone", "CAPS_BOUND", true); err != ErrNotAllowed {
		t.Fatalf("err=%v", err)
	}
}

func TestStatsAndHelloRules(t *testing.T) {
	env, _ := Parse([]byte(`{"v":1,"type":"hello","token":"abc"}`))
	if err := Validate(env, "", "CAPS_BOUND", false); err != nil {
		t.Fatal(err)
	}
	if err := Validate(env, "operator", "NEGOTIATING", true); err != ErrNotAllowed {
		t.Fatalf("second hello: %v", err)
	}
}
