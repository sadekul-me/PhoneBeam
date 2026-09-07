package protocol

import (
	"encoding/json"
	"errors"
	"strings"

	"phonebeam.dev/coordinator/internal/sas"
)

const (
	Version         = 1
	MaxMessageBytes = 64 << 10
	MaxSDPBytes     = 32 << 10
)

var (
	ErrMalformed          = errors.New("malformed signaling message")
	ErrUnknownType        = errors.New("unknown signaling type")
	ErrWrongVersion       = errors.New("unsupported signaling version")
	ErrNotAllowed         = errors.New("message not allowed in this state")
	ErrInvalidSDP         = errors.New("invalid sdp envelope")
	ErrInvalidICE         = errors.New("invalid ice envelope")
	ErrInvalidFingerprint = errors.New("invalid dtls fingerprint")
)

type Envelope struct {
	V           int          `json:"v"`
	Type        string       `json:"type"`
	Token       string       `json:"token,omitempty"`
	SDP         *SDP         `json:"sdp,omitempty"`
	Candidate   *Candidate   `json:"candidate,omitempty"`
	Fingerprint *Fingerprint `json:"fingerprint,omitempty"`
	Projection  *Projection  `json:"projection,omitempty"`
	Stats       *Stats       `json:"stats,omitempty"`
	Error       string       `json:"error,omitempty"`
	State       string       `json:"state,omitempty"`
}

type SDP struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

type Candidate struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdp_mid"`
	SDPMLineIndex int    `json:"sdp_mline_index"`
}

type Fingerprint struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
	Role      string `json:"role"`
}

type Projection struct {
	Status string `json:"status"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Scope  string `json:"scope,omitempty"`
}

type Stats struct {
	Path        string `json:"path,omitempty"`
	BitrateKbps int    `json:"bitrate_kbps,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	RTTMs       int    `json:"rtt_ms,omitempty"`
}

func Parse(raw []byte) (*Envelope, error) {
	if len(raw) == 0 || len(raw) > MaxMessageBytes {
		return nil, ErrMalformed
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, ErrMalformed
	}
	if env.V != Version {
		return nil, ErrWrongVersion
	}
	if env.Type == "" {
		return nil, ErrMalformed
	}
	return &env, nil
}

func Validate(env *Envelope, role string, state string, helloDone bool) error {
	if env == nil {
		return ErrMalformed
	}
	if !helloDone && env.Type != "hello" {
		return ErrNotAllowed
	}
	if helloDone && env.Type == "hello" {
		return ErrNotAllowed
	}
	switch env.Type {
	case "hello":
		if env.Token == "" {
			return ErrMalformed
		}
		return nil
	case "hangup":
		return nil
	case "ice_complete", "peer_ready", "need_offer", "failed_ice", "failed_signaling":
		return requireSignalingState(state)
	case "sdp_offer":
		if role != "phone" {
			return ErrNotAllowed
		}
		if err := requireSignalingState(state); err != nil {
			return err
		}
		return validateSDP(env.SDP, "offer")
	case "sdp_answer":
		if role != "operator" {
			return ErrNotAllowed
		}
		if err := requireSignalingState(state); err != nil {
			return err
		}
		return validateSDP(env.SDP, "answer")
	case "ice_candidate":
		if err := requireSignalingState(state); err != nil {
			return err
		}
		return validateICE(env.Candidate)
	case "peer_fingerprint":
		if err := requireSignalingState(state); err != nil {
			return err
		}
		return validateFP(env.Fingerprint, role)
	case "projection":
		if role != "phone" {
			return ErrNotAllowed
		}
		if env.Projection == nil {
			return ErrMalformed
		}
		switch env.Projection.Status {
		case "pending", "active", "denied":
			return nil
		default:
			return ErrMalformed
		}
	case "stats":
		if role != "phone" && role != "operator" {
			return ErrNotAllowed
		}
		return requireSignalingState(state)
	default:
		return ErrUnknownType
	}
}

func requireSignalingState(state string) error {
	switch state {
	case "PROJECTION_ACTIVE", "NEGOTIATING", "CONNECTED", "RECONNECTING", "FAILED_ICE":
		return nil
	default:
		return ErrNotAllowed
	}
}

func validateSDP(s *SDP, wantType string) error {
	if s == nil || s.Type != wantType || s.SDP == "" || len(s.SDP) > MaxSDPBytes {
		return ErrInvalidSDP
	}
	if !strings.Contains(s.SDP, "v=0") {
		return ErrInvalidSDP
	}
	if strings.Contains(strings.ToLower(s.SDP), "ice-ufrag:") && strings.Contains(s.SDP, "\x00") {
		return ErrInvalidSDP
	}
	return nil
}

func validateICE(c *Candidate) error {
	if c == nil || c.Candidate == "" {
		return ErrInvalidICE
	}
	line := strings.TrimSpace(c.Candidate)
	if !strings.HasPrefix(line, "candidate:") && !strings.HasPrefix(line, "a=candidate:") {
		return ErrInvalidICE
	}
	return nil
}

func validateFP(fp *Fingerprint, role string) error {
	if fp == nil {
		return ErrInvalidFingerprint
	}
	if !strings.EqualFold(fp.Algorithm, "sha-256") {
		return ErrInvalidFingerprint
	}
	if !sas.ValidSHA256Fingerprint(fp.Value) {
		return ErrInvalidFingerprint
	}
	want := role
	if fp.Role != "" && fp.Role != want {
		return ErrInvalidFingerprint
	}
	return nil
}

func ExtractFingerprint(sdp string) (string, bool) {
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "a=fingerprint:sha-256 ") {
			return sas.NormalizeFingerprint(strings.TrimSpace(line[len("a=fingerprint:sha-256 "):])), true
		}
	}
	return "", false
}
