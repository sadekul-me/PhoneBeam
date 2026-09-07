package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"phonebeam.dev/coordinator/internal/protocol"
	"phonebeam.dev/coordinator/internal/session"
)

func approvedProjected(t *testing.T) (coord *session.Coordinator, ts *httptest.Server, sid, op, phone string) {
	t.Helper()
	coord, handler := testServer(t)
	ts = httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	created, err := coord.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := coord.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := coord.Approve(created.Session.ID, scan.PhonePendingToken, session.ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coord.SetProjection(created.Session.ID, approved.PhoneToken, &protocol.Projection{
		Status: "active", Width: 1080, Height: 2400, Scope: "unknown",
	}); err != nil {
		t.Fatal(err)
	}
	return coord, ts, created.Session.ID, created.OperatorToken, approved.PhoneToken
}

func dialSignal(t *testing.T, base, sid, token string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(base, "http") + "/api/v1/sessions/" + sid + "/signal"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.WriteJSON(protocol.Envelope{V: 1, Type: "hello", Token: token}); err != nil {
		t.Fatal(err)
	}
	var hello protocol.Envelope
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&hello); err != nil {
		t.Fatal(err)
	}
	if hello.Type != "hello_ok" {
		t.Fatalf("first message type=%s error=%s", hello.Type, hello.Error)
	}
	return conn
}

func readUntil(t *testing.T, conn *websocket.Conn, want string) protocol.Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 8; i++ {
		var env protocol.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			t.Fatal(err)
		}
		if env.Type == want {
			return env
		}
	}
	t.Fatalf("did not receive %s", want)
	return protocol.Envelope{}
}

func TestWSHelloWrongSessionRejected(t *testing.T) {
	_, ts, sid, _, _ := approvedProjected(t)
	otherCoord := session.NewCoordinator("http://127.0.0.1:8080", time.Minute, time.Hour)
	other, err := otherCoord.Create("Other", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/sessions/" + sid + "/signal"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.WriteJSON(protocol.Envelope{V: 1, Type: "hello", Token: other.OperatorToken})
	var env protocol.Envelope
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatal(err)
	}
	if env.Type != "error" {
		t.Fatalf("wanted error, got %+v", env)
	}
}

func TestWSWrongRoleOfferRejected(t *testing.T) {
	_, ts, sid, op, _ := approvedProjected(t)
	conn := dialSignal(t, ts.URL, sid, op)
	if err := conn.WriteJSON(protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "offer", SDP: "v=0\r\n"}}); err != nil {
		t.Fatal(err)
	}
	env := readUntil(t, conn, "error")
	if env.Error == "" && env.Type != "error" {
		t.Fatalf("operator offer should error, got %+v", env)
	}
}

func TestWSOfferAnswerForward(t *testing.T) {
	_, ts, sid, op, phone := approvedProjected(t)
	opConn := dialSignal(t, ts.URL, sid, op)
	phoneConn := dialSignal(t, ts.URL, sid, phone)
	if err := phoneConn.WriteJSON(protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "offer", SDP: "v=0\r\no=- 1 1 IN IP4 0.0.0.0\r\n"}}); err != nil {
		t.Fatal(err)
	}
	if got := readUntil(t, opConn, "sdp_offer"); got.SDP == nil || got.SDP.Type != "offer" {
		t.Fatalf("offer=%+v", got)
	}
	if err := opConn.WriteJSON(protocol.Envelope{V: 1, Type: "sdp_answer", SDP: &protocol.SDP{Type: "answer", SDP: "v=0\r\n"}}); err != nil {
		t.Fatal(err)
	}
	if got := readUntil(t, phoneConn, "sdp_answer"); got.SDP == nil || got.SDP.Type != "answer" {
		t.Fatalf("answer=%+v", got)
	}
}

func TestWSSignalingBeforeApprovalRejected(t *testing.T) {
	coord, handler := testServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()
	created, err := coord.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/sessions/" + created.Session.ID + "/signal"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.WriteJSON(protocol.Envelope{V: 1, Type: "hello", Token: created.OperatorToken})
	var env protocol.Envelope
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatal(err)
	}
	if env.Type != "error" {
		t.Fatalf("expected error before approval, got %+v", env)
	}
}

func TestWSICERequiresAuthAndReturnsServers(t *testing.T) {
	_, ts, sid, op, _ := approvedProjected(t)
	unauth, err := ts.Client().Get(ts.URL + "/api/v1/sessions/" + sid + "/ice")
	if err != nil {
		t.Fatal(err)
	}
	unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing bearer should 401, got %d", unauth.StatusCode)
	}
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/sessions/"+sid+"/ice", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+op)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("code=%d", resp.StatusCode)
	}
	var cfg struct {
		Servers []json.RawMessage `json:"ice_servers"`
		Policy  string            `json:"ice_transport_policy"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Policy != "all" && cfg.Policy != "relay" {
		t.Fatalf("policy=%s", cfg.Policy)
	}
}

func TestWSHangupAndAfterClose(t *testing.T) {
	coord, ts, sid, op, phone := approvedProjected(t)
	opConn := dialSignal(t, ts.URL, sid, op)
	phoneConn := dialSignal(t, ts.URL, sid, phone)
	if err := opConn.WriteJSON(protocol.Envelope{V: 1, Type: "hangup"}); err != nil {
		t.Fatal(err)
	}
	if got := readUntil(t, phoneConn, "hangup"); got.Type != "hangup" {
		t.Fatalf("hangup=%+v", got)
	}
	got, err := coord.Get(sid, op)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != session.StateClosed {
		t.Fatalf("state=%s", got.State)
	}
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/sessions/" + sid + "/signal"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.WriteJSON(protocol.Envelope{V: 1, Type: "hello", Token: op})
	var env protocol.Envelope
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatal(err)
	}
	if env.Type != "error" {
		t.Fatalf("signaling after close: %+v", env)
	}
}

func TestWSPendingTokenRejected(t *testing.T) {
	coord, handler := testServer(t)
	ts := httptest.NewServer(handler)
	defer ts.Close()
	created, err := coord.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := coord.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/sessions/" + created.Session.ID + "/signal"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.WriteJSON(protocol.Envelope{V: 1, Type: "hello", Token: scan.PhonePendingToken})
	var env protocol.Envelope
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatal(err)
	}
	if env.Type != "error" {
		t.Fatalf("pending token should not signal, got %+v", env)
	}
}

func TestWSICECandidateForward(t *testing.T) {
	_, ts, sid, op, phone := approvedProjected(t)
	opConn := dialSignal(t, ts.URL, sid, op)
	phoneConn := dialSignal(t, ts.URL, sid, phone)
	if err := phoneConn.WriteJSON(protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "offer", SDP: "v=0\r\n"}}); err != nil {
		t.Fatal(err)
	}
	_ = readUntil(t, opConn, "sdp_offer")
	if err := phoneConn.WriteJSON(protocol.Envelope{
		V: 1, Type: "ice_candidate",
		Candidate: &protocol.Candidate{Candidate: "candidate:1 1 UDP 1 127.0.0.1 9 typ host", SDPMid: "0"},
	}); err != nil {
		t.Fatal(err)
	}
	got := readUntil(t, opConn, "ice_candidate")
	if got.Candidate == nil || got.Candidate.Candidate == "" {
		t.Fatalf("candidate=%+v", got)
	}
}

func TestWSRateLimitEventuallyErrors(t *testing.T) {
	_, ts, sid, op, _ := approvedProjected(t)
	conn := dialSignal(t, ts.URL, sid, op)
	sawLimit := false
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < 80; i++ {
		_ = conn.WriteJSON(protocol.Envelope{V: 1, Type: "ice_complete"})
	}
	for i := 0; i < 80; i++ {
		var env protocol.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			break
		}
		if env.Type == "error" && env.Error == "rate_limited" {
			sawLimit = true
			break
		}
	}
	if !sawLimit {
		t.Fatal("expected rate_limited")
	}
}
