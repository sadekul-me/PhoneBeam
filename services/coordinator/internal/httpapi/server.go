package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"phonebeam.dev/coordinator/internal/session"
)

const maxBody = 16 << 10

type Server struct {
	coord    *session.Coordinator
	cors     map[string]struct{}
	limiters sync.Map
	log      *log.Logger
	socks    *sockets
}

func New(coord *session.Coordinator, corsOrigins []string, logger *log.Logger) *Server {
	allowed := map[string]struct{}{}
	for _, origin := range corsOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return &Server{coord: coord, cors: allowed, log: logger, socks: newSockets()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /api/v1/sessions", s.create)
	mux.HandleFunc("GET /api/v1/sessions/{id}", s.get)
	mux.HandleFunc("POST /api/v1/sessions/{id}/scan", s.scan)
	mux.HandleFunc("POST /api/v1/sessions/{id}/approve", s.approve)
	mux.HandleFunc("POST /api/v1/sessions/{id}/reject", s.reject)
	mux.HandleFunc("POST /api/v1/sessions/{id}/close", s.close)
	mux.HandleFunc("POST /api/v1/sessions/{id}/projection", s.projection)
	mux.HandleFunc("GET /api/v1/sessions/{id}/ice", s.ice)
	mux.HandleFunc("GET /api/v1/sessions/{id}/signal", s.signal)
	return s.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := s.cors[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			} else if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if !s.allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "phonebeam-coordinator"})
}

type createRequest struct {
	OperatorDisplayName   string   `json:"operator_display_name"`
	RequestedCapabilities []string `json:"requested_capabilities"`
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid json")
		return
	}
	result, err := s.coord.Create(req.OperatorDisplayName, req.RequestedCapabilities)
	if err != nil {
		s.writeCoordError(w, err)
		return
	}
	s.log.Printf("session created id=%s state=%s", result.Session.ID, result.Session.State)
	writeJSON(w, http.StatusCreated, map[string]any{
		"session":        result.Session,
		"operator_token": result.OperatorToken,
	})
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, err := bearer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	pub, err := s.coord.Get(id, token)
	if err != nil {
		s.writeCoordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": pub})
}

type scanRequest struct {
	PID string `json:"pid"`
}

func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req scanRequest
	if err := decodeJSON(r, &req); err != nil || req.PID == "" {
		writeError(w, http.StatusBadRequest, "invalid_json", "pid required")
		return
	}
	result, err := s.coord.Scan(id, req.PID)
	if err != nil {
		s.writeCoordError(w, err)
		return
	}
	s.log.Printf("session scanned id=%s state=%s", result.Session.ID, result.Session.State)
	writeJSON(w, http.StatusOK, map[string]any{
		"session":             result.Session,
		"phone_pending_token": result.PhonePendingToken,
	})
}

type approveRequest struct {
	GrantedCapabilities []string `json:"granted_capabilities"`
	DeviceCapabilities  []string `json:"device_capabilities"`
}

func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, err := bearer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	var req approveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid json")
		return
	}
	result, err := s.coord.Approve(id, token, session.ApproveInput{
		Granted: req.GrantedCapabilities,
		Device:  req.DeviceCapabilities,
	})
	if err != nil {
		s.writeCoordError(w, err)
		return
	}
	s.log.Printf("session approved id=%s state=%s", result.Session.ID, result.Session.State)
	writeJSON(w, http.StatusOK, map[string]any{
		"session":     result.Session,
		"phone_token": result.PhoneToken,
	})
}

func (s *Server) reject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, err := bearer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	pub, err := s.coord.Reject(id, token)
	if err != nil {
		s.writeCoordError(w, err)
		return
	}
	s.log.Printf("session rejected id=%s state=%s", pub.ID, pub.State)
	writeJSON(w, http.StatusOK, map[string]any{"session": pub})
}

func (s *Server) close(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, err := bearer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	pub, err := s.coord.Close(id, token)
	if err != nil {
		s.writeCoordError(w, err)
		return
	}
	s.log.Printf("session closed id=%s state=%s", pub.ID, pub.State)
	writeJSON(w, http.StatusOK, map[string]any{"session": pub})
}

func (s *Server) writeCoordError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, session.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "session not found")
	case errors.Is(err, session.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
	case errors.Is(err, session.ErrExpired):
		writeError(w, http.StatusGone, "expired", "pairing expired")
	case errors.Is(err, session.ErrScanLocked):
		writeError(w, http.StatusConflict, "rejected_consumed", "pairing already scanned")
	case errors.Is(err, session.ErrReplay):
		writeError(w, http.StatusConflict, "replay", "pairing already consumed")
	case errors.Is(err, session.ErrUnrequestedGrant):
		writeError(w, http.StatusBadRequest, "unrequested_capability", "cannot grant unrequested capability")
	case errors.Is(err, session.ErrUnknownCapability):
		writeError(w, http.StatusBadRequest, "unknown_capability", "unknown capability")
	case errors.Is(err, session.ErrClosed), errors.Is(err, session.ErrTerminal):
		writeError(w, http.StatusConflict, "terminal", "session cannot be reused")
	case errors.Is(err, session.ErrPairingNotPending), errors.Is(err, session.ErrWrongPairing):
		writeError(w, http.StatusConflict, "invalid_state", err.Error())
	case errors.Is(err, session.ErrSignalingNotAllowed):
		writeError(w, http.StatusConflict, "signaling_not_allowed", err.Error())
	case errors.Is(err, session.ErrMissingScreenRead):
		writeError(w, http.StatusBadRequest, "screen_read_required", err.Error())
	case errors.Is(err, session.ErrRoleExclusive):
		writeError(w, http.StatusConflict, "role_exclusive", err.Error())
	default:
		var invalid *session.InvalidTransitionError
		if errors.As(err, &invalid) {
			writeError(w, http.StatusConflict, "invalid_transition", invalid.Error())
			return
		}
		s.log.Printf("internal error")
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dest)
}

func bearer(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", errors.New("missing")
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	if token == "" {
		return "", errors.New("missing")
	}
	return token, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type limiter struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func (s *Server) allow(ip string) bool {
	now := time.Now()
	v, _ := s.limiters.LoadOrStore(ip, &limiter{tokens: 20, last: now})
	lim := v.(*limiter)
	lim.mu.Lock()
	defer lim.mu.Unlock()
	elapsed := now.Sub(lim.last).Seconds()
	lim.tokens += elapsed * 5
	if lim.tokens > 20 {
		lim.tokens = 20
	}
	lim.last = now
	if lim.tokens < 1 {
		return false
	}
	lim.tokens--
	return true
}
