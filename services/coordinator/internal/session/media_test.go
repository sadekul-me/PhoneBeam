package session

import (
	"testing"
	"time"

	"phonebeam.dev/coordinator/internal/protocol"
	"phonebeam.dev/coordinator/internal/sas"
)

func fixtureCaps(t *testing.T) (c *Coordinator, sid, op, phone string) {
	t.Helper()
	c = NewCoordinator("http://127.0.0.1:8080", 120*time.Second, 60*time.Minute)
	created, err := c.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := c.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := c.Approve(created.Session.ID, scan.PhonePendingToken, ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c, created.Session.ID, created.OperatorToken, approved.PhoneToken
}

func TestPhonePendingCannotAttachSignaling(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", time.Minute, time.Hour)
	created, err := c.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := c.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.AttachSignal(created.Session.ID, scan.PhonePendingToken); err != ErrUnauthorized && err != ErrSignalingNotAllowed {
		t.Fatalf("pending token attached: %v", err)
	}
}

func TestSignalingBeforeApprovalRejected(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", time.Minute, time.Hour)
	created, err := c.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.AttachSignal(created.Session.ID, created.OperatorToken); err != ErrSignalingNotAllowed {
		t.Fatalf("err=%v", err)
	}
}

func TestWrongRoleAndSessionRejected(t *testing.T) {
	c, sid, op, phone := fixtureCaps(t)
	other, err := c.Create("Op2", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.AttachSignal(sid, other.OperatorToken); err != ErrUnauthorized {
		t.Fatalf("cross-session token: %v", err)
	}
	if _, _, err := c.AttachSignal(sid, "nope"); err != ErrUnauthorized {
		t.Fatalf("bad token: %v", err)
	}
	_ = op
	_ = phone
}

func TestCannotConnectOnSDPAlone(t *testing.T) {
	c, sid, op, phone := fixtureCaps(t)
	if _, _, err := c.AttachSignal(sid, phone); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "pending"}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "active", Width: 1080, Height: 2400, Scope: "unknown"}}); err != nil {
		t.Fatal(err)
	}
	offer := &protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "offer", SDP: "v=0\r\no=- 1 1 IN IP4 0.0.0.0\r\n"}}
	pub, _, err := c.HandleSignal(sid, RolePhone, offer)
	if err != nil {
		t.Fatal(err)
	}
	if pub.State != StateNegotiating {
		t.Fatalf("state=%s", pub.State)
	}
	if _, _, err := c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "sdp_answer", SDP: &protocol.SDP{Type: "answer", SDP: "v=0\r\n"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := c.Get(sid, op)
	if got.State == StateConnected {
		t.Fatal("CONNECTED without DTLS bind")
	}
}

func TestConnectedRequiresFingerprintsAndReady(t *testing.T) {
	c, sid, op, phone := fixtureCaps(t)
	_, _, _ = c.AttachSignal(sid, phone)
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "pending"}})
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "active"}})
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "offer", SDP: "v=0\r\n"}})
	_, _, _ = c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "sdp_answer", SDP: &protocol.SDP{Type: "answer", SDP: "v=0\r\n"}})
	fpA := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	fpB := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "peer_fingerprint", Fingerprint: &protocol.Fingerprint{Algorithm: "sha-256", Value: fpA, Role: "phone"}})
	_, _, _ = c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "peer_fingerprint", Fingerprint: &protocol.Fingerprint{Algorithm: "sha-256", Value: fpB, Role: "operator"}})
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "peer_ready"})
	pub, _, err := c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "peer_ready"})
	if err != nil {
		t.Fatal(err)
	}
	if pub.State != StateConnected || !pub.MediaTrusted {
		t.Fatalf("state=%s trusted=%v", pub.State, pub.MediaTrusted)
	}
	material, _ := c.Get(sid, op)
	media := sas.MediaDisplay(c.SASKey(), c.Origin(), sid, "", "Op", []string{"screen.read"}, fpA, fpB)
	if media == material.SAS {
		t.Fatal("pairing SAS must remain distinct from media SAS")
	}
}

func TestHangupAndSignalingAfterClose(t *testing.T) {
	c, sid, op, phone := fixtureCaps(t)
	_, _, _ = c.AttachSignal(sid, phone)
	if _, err := c.Close(sid, op); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "hangup"}); err != ErrTerminal && err != ErrClosed {
		t.Fatalf("err=%v", err)
	}
}

func TestProjectionDeniedState(t *testing.T) {
	c, sid, op, phone := fixtureCaps(t)
	_, _, _ = c.AttachSignal(sid, phone)
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "pending"}})
	pub, _, err := c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "denied"}})
	if err != nil {
		t.Fatal(err)
	}
	if pub.State != StateProjectionDenied {
		t.Fatalf("state=%s", pub.State)
	}
	got, _ := c.Get(sid, op)
	if got.State != StateProjectionDenied {
		t.Fatalf("op view %s", got.State)
	}
}

func TestInvalidSignalingRejected(t *testing.T) {
	c, sid, _, phone := fixtureCaps(t)
	_, _, _ = c.AttachSignal(sid, phone)
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "active"}})
	if _, _, err := c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "answer", SDP: "v=0\r\n"}}); err == nil {
		t.Fatal("malformed offer envelope accepted")
	}
	if _, _, err := c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "offer", SDP: "v=0\r\n"}}); err == nil {
		t.Fatal("operator offer accepted")
	}
}

func TestICEAndHangupForward(t *testing.T) {
	c, sid, op, phone := fixtureCaps(t)
	_, _, _ = c.AttachSignal(sid, phone)
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "active"}})
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "offer", SDP: "v=0\r\n"}})
	_, fwd, err := c.HandleSignal(sid, RolePhone, &protocol.Envelope{
		V: 1, Type: "ice_candidate", Token: "must-strip",
		Candidate: &protocol.Candidate{Candidate: "candidate:1 1 UDP 1 127.0.0.1 9 typ host", SDPMid: "0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fwd == nil || fwd.Token != "" || fwd.Type != "ice_candidate" {
		t.Fatalf("forward=%+v", fwd)
	}
	pub, _, err := c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "hangup"})
	if err != nil {
		t.Fatal(err)
	}
	if pub.State != StateClosed {
		t.Fatalf("state=%s", pub.State)
	}
	if _, _, err := c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "ice_complete"}); err != ErrTerminal && err != ErrClosed {
		t.Fatalf("signaling after close: %v", err)
	}
	_ = op
}

func TestExpiryClosesSignaling(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", 120*time.Second, 50*time.Millisecond)
	now := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return now }
	created, err := c.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := c.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := c.Approve(created.Session.ID, scan.PhonePendingToken, ApproveInput{
		Granted: []string{"screen.read"}, Device: []string{"screen.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(60 * time.Millisecond)
	if _, _, err := c.AttachSignal(created.Session.ID, approved.PhoneToken); err != ErrTerminal && err != ErrClosed {
		t.Fatalf("expired attach: %v", err)
	}
}

func TestOneRehandshakeThenClose(t *testing.T) {
	c, sid, op, phone := fixtureCaps(t)
	_, _, _ = c.AttachSignal(sid, phone)
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "projection", Projection: &protocol.Projection{Status: "active"}})
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "sdp_offer", SDP: &protocol.SDP{Type: "offer", SDP: "v=0\r\n"}})
	_, _, _ = c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "sdp_answer", SDP: &protocol.SDP{Type: "answer", SDP: "v=0\r\n"}})
	fpA := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	fpB := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "peer_fingerprint", Fingerprint: &protocol.Fingerprint{Algorithm: "sha-256", Value: fpA, Role: "phone"}})
	_, _, _ = c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "peer_fingerprint", Fingerprint: &protocol.Fingerprint{Algorithm: "sha-256", Value: fpB, Role: "operator"}})
	_, _, _ = c.HandleSignal(sid, RolePhone, &protocol.Envelope{V: 1, Type: "peer_ready"})
	_, _, _ = c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "peer_ready"})
	pub, _, err := c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "need_offer"})
	if err != nil {
		t.Fatal(err)
	}
	if pub.State != StateReconnecting {
		t.Fatalf("state=%s", pub.State)
	}
	pub, _, err = c.HandleSignal(sid, RoleOperator, &protocol.Envelope{V: 1, Type: "need_offer"})
	if err != nil {
		t.Fatal(err)
	}
	if pub.State != StateClosed {
		t.Fatalf("second rehandshake state=%s", pub.State)
	}
	_ = op
}

func TestM2HappyPathMachine(t *testing.T) {
	state := StateCapsBound
	var err error
	for _, event := range []Event{
		EventProjectionPending, EventProjectionActive, EventBeginNegotiate, EventConnected, EventClose, EventFinishClose,
	} {
		state, err = Apply(state, event)
		if err != nil {
			t.Fatalf("%s: %v", event, err)
		}
	}
	if state != StateClosed {
		t.Fatalf("state=%s", state)
	}
	if _, err := Apply(StateCapsBound, EventConnected); err == nil {
		t.Fatal("CONNECTED from CAPS_BOUND")
	}
}
