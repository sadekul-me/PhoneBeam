package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"phonebeam.dev/coordinator/internal/protocol"
	"phonebeam.dev/coordinator/internal/session"
)

type safeConn struct {
	mu sync.Mutex
	c  *websocket.Conn
}

func (s *safeConn) writeJSON(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return s.c.WriteJSON(v)
}

type sockets struct {
	mu    sync.Mutex
	conns map[string]map[session.Role]*safeConn
}

func newSockets() *sockets {
	return &sockets{conns: map[string]map[session.Role]*safeConn{}}
}

func (s *sockets) put(id string, role session.Role, conn *safeConn) *safeConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns[id] == nil {
		s.conns[id] = map[session.Role]*safeConn{}
	}
	prev := s.conns[id][role]
	s.conns[id][role] = conn
	return prev
}

func (s *sockets) get(id string, role session.Role) *safeConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns[id] == nil {
		return nil
	}
	return s.conns[id][role]
}

func (s *sockets) drop(id string, role session.Role, conn *safeConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns[id] != nil && s.conns[id][role] == conn {
		delete(s.conns[id], role)
	}
}

func (s *Server) projection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, err := bearer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	var req protocol.Projection
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid json")
		return
	}
	pub, err := s.coord.SetProjection(id, token, &req)
	if err != nil {
		s.writeCoordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": pub})
}

func (s *Server) ice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token, err := bearer(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return
	}
	cfg, err := s.coord.ICEConfig(id, token)
	if err != nil {
		s.writeCoordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) signal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(req *http.Request) bool {
			origin := req.Header.Get("Origin")
			if origin == "" {
				return true
			}
			_, ok := s.cors[origin]
			return ok
		},
	}
	rawConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn := &safeConn{c: rawConn}
	rawConn.SetReadLimit(protocol.MaxMessageBytes)
	_ = rawConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := rawConn.ReadMessage()
	if err != nil {
		_ = rawConn.Close()
		return
	}
	env, err := protocol.Parse(raw)
	if err != nil || env.Type != "hello" {
		_ = conn.writeJSON(protocol.Envelope{V: 1, Type: "error", Error: "hello_required"})
		_ = rawConn.Close()
		return
	}
	if err := protocol.Validate(env, "", string(session.StateCapsBound), false); err != nil {
		_ = conn.writeJSON(protocol.Envelope{V: 1, Type: "error", Error: "invalid_hello"})
		_ = rawConn.Close()
		return
	}
	role, pub, err := s.coord.AttachSignal(id, env.Token)
	if err != nil {
		_ = conn.writeJSON(protocol.Envelope{V: 1, Type: "error", Error: err.Error()})
		_ = rawConn.Close()
		return
	}
	if prev := s.socks.put(id, role, conn); prev != nil {
		_ = prev.c.Close()
	}
	_ = rawConn.SetReadDeadline(time.Time{})
	_ = conn.writeJSON(protocol.Envelope{V: 1, Type: "hello_ok", State: string(pub.State)})

	limiter := &limiter{tokens: 30, last: time.Now()}
	for {
		_, msg, err := rawConn.ReadMessage()
		if err != nil {
			break
		}
		if !s.consumeWS(limiter) {
			_ = conn.writeJSON(protocol.Envelope{V: 1, Type: "error", Error: "rate_limited"})
			continue
		}
		parsed, err := protocol.Parse(msg)
		if err != nil {
			_ = conn.writeJSON(protocol.Envelope{V: 1, Type: "error", Error: err.Error()})
			continue
		}
		pub, forward, err := s.coord.HandleSignal(id, role, parsed)
		if err != nil {
			_ = conn.writeJSON(protocol.Envelope{V: 1, Type: "error", Error: err.Error()})
			continue
		}
		state := protocol.Envelope{V: 1, Type: "state", State: string(pub.State)}
		_ = conn.writeJSON(state)
		if peer := s.socks.get(id, otherRole(role)); peer != nil {
			_ = peer.writeJSON(state)
			if shouldForward(forward) {
				_ = peer.writeJSON(forward)
			}
		}
		if pub.State == session.StateClosed {
			break
		}
	}
	s.socks.drop(id, role, conn)
	s.coord.DetachSignal(id, role)
	if role == session.RolePhone {
		_, _, _ = s.coord.HandleSignal(id, role, &protocol.Envelope{V: 1, Type: "hangup"})
	} else {
		token := env.Token
		time.AfterFunc(5*time.Second, func() {
			if s.coord.RoleAttached(id, session.RoleOperator) {
				return
			}
			got, err := s.coord.Get(id, token)
			if err != nil {
				return
			}
			if got.State != session.StateClosed && got.State != session.StateExpired {
				_, _, _ = s.coord.HandleSignal(id, session.RoleOperator, &protocol.Envelope{V: 1, Type: "hangup"})
			}
		})
	}
	_ = rawConn.Close()
}

func shouldForward(env *protocol.Envelope) bool {
	if env == nil {
		return false
	}
	switch env.Type {
	case "sdp_offer", "sdp_answer", "ice_candidate", "ice_complete", "need_offer", "hangup":
		return true
	default:
		return false
	}
}

func otherRole(role session.Role) session.Role {
	if role == session.RolePhone {
		return session.RoleOperator
	}
	return session.RolePhone
}

func (s *Server) consumeWS(lim *limiter) bool {
	lim.mu.Lock()
	defer lim.mu.Unlock()
	now := time.Now()
	lim.tokens += now.Sub(lim.last).Seconds() * 10
	if lim.tokens > 30 {
		lim.tokens = 30
	}
	lim.last = now
	if lim.tokens < 1 {
		return false
	}
	lim.tokens--
	return true
}
