package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"phonebeam.dev/coordinator/internal/httpapi"
	"phonebeam.dev/coordinator/internal/session"
)

func main() {
	listen := env("PHONEBEAM_LISTEN", "127.0.0.1:8080")
	origin := env("PHONEBEAM_ORIGIN", "http://127.0.0.1:8080")
	cors := strings.Split(env("PHONEBEAM_CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"), ",")
	qrTTL := durationEnv("PHONEBEAM_QR_TTL", 120*time.Second)
	sessionTTL := durationEnv("PHONEBEAM_SESSION_TTL", 60*time.Minute)

	logger := log.New(os.Stdout, "", log.LstdFlags)
	coord := session.NewCoordinator(origin, qrTTL, sessionTTL)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			coord.Sweep()
		}
	}()
	srv := httpapi.New(coord, cors, logger)
	logger.Printf("phonebeam coordinator listening on %s origin=%s qr_ttl=%s session_ttl=%s", listen, origin, qrTTL, sessionTTL)
	if err := http.ListenAndServe(listen, srv.Handler()); err != nil {
		logger.Fatal(err)
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
	}
	return fallback
}
