package ice

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

type Server struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type Config struct {
	Servers         []Server  `json:"ice_servers"`
	TransportPolicy string    `json:"ice_transport_policy"`
	TurnConfigured  bool      `json:"turn_configured"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type Options struct {
	STUNURIs        []string
	TURNURIs        []string
	TURNSecret      string
	TTL             time.Duration
	TransportPolicy string
	Now             time.Time
}

func Build(sid string, opt Options) Config {
	policy := opt.TransportPolicy
	if policy != "relay" {
		policy = "all"
	}
	ttl := opt.TTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	cfg := Config{TransportPolicy: policy, ExpiresAt: opt.Now.Add(ttl)}
	for _, uri := range opt.STUNURIs {
		uri = strings.TrimSpace(uri)
		if uri != "" {
			cfg.Servers = append(cfg.Servers, Server{URLs: []string{uri}})
		}
	}
	secret := strings.TrimSpace(opt.TURNSecret)
	if secret != "" && len(opt.TURNURIs) > 0 {
		user, pass := RESTCredentials(secret, sid, opt.Now.Add(ttl))
		cfg.Servers = append(cfg.Servers, Server{
			URLs:       append([]string{}, opt.TURNURIs...),
			Username:   user,
			Credential: pass,
		})
		cfg.TurnConfigured = true
	}
	return cfg
}

// RESTCredentials implements coturn --use-auth-secret (TURN REST API).
func RESTCredentials(secret, sid string, expiry time.Time) (username, password string) {
	username = fmt.Sprintf("%d:%s", expiry.Unix(), sid)
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(username))
	password = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return username, password
}
