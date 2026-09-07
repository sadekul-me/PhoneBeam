package sas

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"sort"
	"strings"
)

const (
	Domain   = "phonebeam-sas-v1"
	DomainV2 = "phonebeam-sas-v2"
)

func writePairingTranscript(mac hash.Hash, origin, sid, pid, operatorID string, requestedCaps []string) {
	caps := append([]string{}, requestedCaps...)
	sort.Strings(caps)
	_, _ = mac.Write([]byte("origin=" + origin + "\n"))
	_, _ = mac.Write([]byte("sid=" + sid + "\n"))
	_, _ = mac.Write([]byte("pid=" + pid + "\n"))
	_, _ = mac.Write([]byte("operator=" + operatorID + "\n"))
	_, _ = mac.Write([]byte("caps=" + strings.Join(caps, ",") + "\n"))
}

func digits(sum []byte) string {
	n := binary.BigEndian.Uint32(sum[:4]) % 1_000_000
	return fmt.Sprintf("%03d %03d", n/1000, n%1000)
}

// Display is the M1 pairing SAS. It does not bind WebRTC DTLS fingerprints.
func Display(key []byte, origin, sid, pid, operatorID string, requestedCaps []string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(Domain + "\n"))
	writePairingTranscript(mac, origin, sid, pid, operatorID, requestedCaps)
	return digits(mac.Sum(nil))
}

// MediaDisplay is the M2 media-authentication SAS.
// Each peer must compute this locally using fingerprints observed on its own PeerConnection
// (local certificate + remote DTLS), never a coordinator-supplied fingerprint pair.
func MediaDisplay(key []byte, origin, sid, pid, operatorID string, requestedCaps []string, androidFP, browserFP string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(DomainV2 + "\n"))
	writePairingTranscript(mac, origin, sid, pid, operatorID, requestedCaps)
	_, _ = mac.Write([]byte("android_fp=" + NormalizeFingerprint(androidFP) + "\n"))
	_, _ = mac.Write([]byte("browser_fp=" + NormalizeFingerprint(browserFP) + "\n"))
	return digits(mac.Sum(nil))
}

// NormalizeFingerprint lowercases SHA-256 hex and strips colons/spaces.
func NormalizeFingerprint(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ValidSHA256Fingerprint(value string) bool {
	n := NormalizeFingerprint(value)
	return len(n) == 64
}
