package ice

import (
	"strings"
	"testing"
	"time"
)

func TestRESTCredentialsStable(t *testing.T) {
	user, pass := RESTCredentials("dev-secret", "sid1", time.Unix(1_700_000_000, 0))
	if user != "1700000000:sid1" {
		t.Fatalf("user=%s", user)
	}
	if pass == "" || pass == "dev-secret" {
		t.Fatal("password must be hmac, not the secret")
	}
}

func TestBuildWithoutSecretHasNoTURN(t *testing.T) {
	cfg := Build("sid", Options{
		STUNURIs: []string{"stun:stun.example:3478"},
		TURNURIs: []string{"turn:turn.example:3478?transport=udp"},
		Now:      time.Unix(1, 0),
	})
	if cfg.TurnConfigured {
		t.Fatal("turn must not be marked configured without secret")
	}
	if cfg.TransportPolicy != "all" {
		t.Fatalf("policy=%s", cfg.TransportPolicy)
	}
}

func TestBuildWithSecretIsSessionScoped(t *testing.T) {
	cfg := Build("sid-9", Options{
		STUNURIs:   []string{"stun:stun.example:3478"},
		TURNURIs:   []string{"turn:turn.example:3478?transport=udp", "turns:turn.example:443?transport=tcp"},
		TURNSecret: "dev-secret",
		Now:        time.Unix(1_700_000_000, 0),
		TTL:        time.Minute,
	})
	if !cfg.TurnConfigured {
		t.Fatal("expected turn_configured")
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("servers=%d", len(cfg.Servers))
	}
	turn := cfg.Servers[1]
	if turn.Username != "1700000060:sid-9" {
		t.Fatalf("user=%s", turn.Username)
	}
	if strings.Contains(turn.Credential, "dev-secret") {
		t.Fatal("secret leaked into credential")
	}
}
