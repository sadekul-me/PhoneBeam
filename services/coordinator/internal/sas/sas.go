package sas

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

const Domain = "phonebeam-sas-v1"

// Display returns a 6-digit grouped code derived from HMAC-SHA256 of the pairing transcript.
// M1 binds pairing/session identity only. M2 must mix WebRTC DTLS fingerprints into this transcript
// before media is trusted.
func Display(key []byte, origin, sid, pid, operatorID string, requestedCaps []string) string {
	caps := append([]string{}, requestedCaps...)
	sort.Strings(caps)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(Domain + "\n"))
	_, _ = mac.Write([]byte("origin=" + origin + "\n"))
	_, _ = mac.Write([]byte("sid=" + sid + "\n"))
	_, _ = mac.Write([]byte("pid=" + pid + "\n"))
	_, _ = mac.Write([]byte("operator=" + operatorID + "\n"))
	_, _ = mac.Write([]byte("caps=" + strings.Join(caps, ",") + "\n"))
	sum := mac.Sum(nil)
	n := binary.BigEndian.Uint32(sum[:4]) % 1_000_000
	return fmt.Sprintf("%03d %03d", n/1000, n%1000)
}
