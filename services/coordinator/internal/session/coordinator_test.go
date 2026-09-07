package session

import (
	"sync"
	"testing"
	"time"
)

func TestApplyRejectsMediaShortcuts(t *testing.T) {
	cases := []struct {
		from  State
		event Event
	}{
		{StateQRAvailable, EventApprove},
		{StateQRAvailable, EventBindCaps},
		{StatePhoneScanned, EventApprove},
		{StateApprovalPending, EventIssueQR},
		{StateClosed, EventIssueQR},
		{StateExpired, EventScan},
		{StateRejected, EventApprove},
		{StateCapsBound, EventScan},
		{StateCapsBound, EventIssueQR},
	}
	for _, tc := range cases {
		if _, err := Apply(tc.from, tc.event); err == nil {
			t.Fatalf("expected reject %s + %s", tc.from, tc.event)
		}
	}
}

func TestCreateScanApproveClose(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", 120*time.Second, 60*time.Minute)
	created, err := c.Create("Support-A", []string{"screen.read", "input.control"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Session.State != StateQRAvailable || created.Session.QR == nil {
		t.Fatalf("qr not issued: %+v", created.Session)
	}
	if created.OperatorToken == "" || created.Session.SAS == "" {
		t.Fatal("missing operator token or sas")
	}
	scan, err := c.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Session.State != StateApprovalPending {
		t.Fatalf("state=%s", scan.Session.State)
	}
	if scan.Session.SAS != created.Session.SAS {
		t.Fatal("sas mismatch between operator and phone")
	}
	approved, err := c.Approve(created.Session.ID, scan.PhonePendingToken, ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read", "input.control"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Session.State != StateCapsBound {
		t.Fatalf("state=%s", approved.Session.State)
	}
	if got := approved.Session.EffectiveCapabilities; len(got) != 1 || got[0] != "screen.read" {
		t.Fatalf("effective=%v", got)
	}
	if approved.Session.SessionExpiresAt == nil {
		t.Fatal("session ttl should start at approval")
	}
	closed, err := c.Close(created.Session.ID, created.OperatorToken)
	if err != nil {
		t.Fatal(err)
	}
	if closed.State != StateClosed {
		t.Fatalf("state=%s", closed.State)
	}
	if _, err := c.Close(created.Session.ID, created.OperatorToken); err == nil {
		t.Fatal("reopen/close of closed session must fail")
	}
}

func TestQRExpires(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", 120*time.Second, 60*time.Minute)
	now := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return now }
	created, err := c.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(121 * time.Second)
	if _, err := c.Scan(created.Session.ID, created.Session.QR.PID); err != ErrExpired {
		t.Fatalf("err=%v", err)
	}
	got, err := c.Get(created.Session.ID, created.OperatorToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateExpired {
		t.Fatalf("state=%s", got.State)
	}
}

func TestSecondScanRejectedWithoutAbortingFirst(t *testing.T) {
	fx := fixturePending(t)
	if _, err := fx.coord.Scan(fx.sid, fx.pid); err != ErrScanLocked {
		t.Fatalf("err=%v", err)
	}
	got, err := fx.coord.Get(fx.sid, fx.pending)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateApprovalPending {
		t.Fatalf("winner aborted: %s", got.State)
	}
}

func TestConcurrentDoubleScan(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", 120*time.Second, 60*time.Minute)
	created, err := c.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, scanErr := c.Scan(created.Session.ID, created.Session.QR.PID)
			errs <- scanErr
		}()
	}
	wg.Wait()
	close(errs)
	ok, locked := 0, 0
	for err := range errs {
		switch err {
		case nil:
			ok++
		case ErrScanLocked:
			locked++
		default:
			t.Fatalf("unexpected %v", err)
		}
	}
	if ok != 1 || locked != 1 {
		t.Fatalf("ok=%d locked=%d", ok, locked)
	}
}

func TestRejectAndReplay(t *testing.T) {
	fx := fixturePending(t)
	if _, err := fx.coord.Reject(fx.sid, fx.pending); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.coord.Scan(fx.sid, fx.pid); err != ErrReplay && err != ErrTerminal {
		t.Fatalf("err=%v", err)
	}
}

func TestCannotGrantUnrequested(t *testing.T) {
	fx := fixturePending(t)
	_, err := fx.coord.Approve(fx.sid, fx.pending, ApproveInput{
		Granted: []string{"audio.read"},
		Device:  []string{"audio.read"},
	})
	if err != ErrUnrequestedGrant {
		t.Fatalf("err=%v", err)
	}
}

func TestUnavailableDeviceCapabilityExcluded(t *testing.T) {
	fx := fixturePending(t)
	approved, err := fx.coord.Approve(fx.sid, fx.pending, ApproveInput{
		Granted: []string{"screen.read", "input.control"},
		Device:  []string{"screen.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(approved.Session.EffectiveCapabilities) != 1 || approved.Session.EffectiveCapabilities[0] != "screen.read" {
		t.Fatalf("effective=%v", approved.Session.EffectiveCapabilities)
	}
}

func TestUnknownCapabilityRejectedOnCreate(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", time.Minute, time.Hour)
	if _, err := c.Create("Op", []string{"root.shell"}); err != ErrUnknownCapability {
		t.Fatalf("err=%v", err)
	}
}

func TestRoleTokensAreSeparated(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", time.Minute, time.Hour)
	created, err := c.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := c.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Approve(created.Session.ID, created.OperatorToken, ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read"},
	}); err != ErrUnauthorized {
		t.Fatalf("operator approved with operator token: %v", err)
	}
	approved, err := c.Approve(created.Session.ID, scan.PhonePendingToken, ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(created.Session.ID, scan.PhonePendingToken); err != ErrUnauthorized {
		t.Fatal("pending token must die after approve")
	}
	if _, err := c.Get(created.Session.ID, approved.PhoneToken); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredPairingCannotApprove(t *testing.T) {
	fx := fixturePending(t)
	now := fx.coord.now().Add(121 * time.Second)
	fx.coord.now = func() time.Time { return now }
	_, err := fx.coord.Approve(fx.sid, fx.pending, ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read"},
	})
	if err != ErrPairingNotPending && err != ErrExpired && err != ErrTerminal {
		t.Fatalf("err=%v", err)
	}
}

func TestSessionTTLStartsAtApproval(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", 120*time.Second, 60*time.Minute)
	start := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return start }
	created, err := c.Create("Op", []string{"screen.read"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Session.SessionExpiresAt != nil {
		t.Fatal("session ttl must not start at QR create")
	}
	c.now = func() time.Time { return start.Add(30 * time.Second) }
	scan, err := c.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	approveAt := start.Add(40 * time.Second)
	c.now = func() time.Time { return approveAt }
	approved, err := c.Approve(created.Session.ID, scan.PhonePendingToken, ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Session.SessionExpiresAt == nil || !approved.Session.SessionExpiresAt.Equal(approveAt.Add(60*time.Minute)) {
		t.Fatalf("ttl=%v", approved.Session.SessionExpiresAt)
	}
	c.now = func() time.Time { return approveAt.Add(60*time.Minute + time.Second) }
	got, err := c.Get(created.Session.ID, created.OperatorToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateClosed {
		t.Fatalf("expected CLOSED after session ttl, got %s", got.State)
	}
}

func TestMalformedCreateRejected(t *testing.T) {
	c := NewCoordinator("http://127.0.0.1:8080", time.Minute, time.Hour)
	if _, err := c.Create("Op", nil); err != ErrUnknownCapability {
		t.Fatalf("err=%v", err)
	}
}

func TestApproveBurnsPairingReplay(t *testing.T) {
	fx := fixturePending(t)
	if _, err := fx.coord.Approve(fx.sid, fx.pending, ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.coord.Scan(fx.sid, fx.pid); err != ErrReplay && err != ErrTerminal {
		t.Fatalf("err=%v", err)
	}
}

func TestOwnerGrantSubset(t *testing.T) {
	fx := fixturePending(t)
	approved, err := fx.coord.Approve(fx.sid, fx.pending, ApproveInput{
		Granted: []string{"screen.read"},
		Device:  []string{"screen.read", "input.control"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(approved.Session.GrantedCapabilities) != 1 || approved.Session.GrantedCapabilities[0] != "screen.read" {
		t.Fatalf("granted=%v", approved.Session.GrantedCapabilities)
	}
	if len(approved.Session.EffectiveCapabilities) != 1 {
		t.Fatalf("effective=%v", approved.Session.EffectiveCapabilities)
	}
}

func TestHappyPathTransitions(t *testing.T) {
	state := StateSessionCreated
	var err error
	for _, event := range []Event{EventIssueQR, EventScan, EventBeginApproval, EventApprove, EventBindCaps, EventClose, EventFinishClose} {
		state, err = Apply(state, event)
		if err != nil {
			t.Fatalf("%s: %v", event, err)
		}
	}
	if state != StateClosed {
		t.Fatalf("state=%s", state)
	}
}

type pendingFix struct {
	coord   *Coordinator
	sid     string
	pid     string
	pending string
}

func fixturePending(t *testing.T) pendingFix {
	t.Helper()
	c := NewCoordinator("http://127.0.0.1:8080", 120*time.Second, 60*time.Minute)
	created, err := c.Create("Op", []string{"screen.read", "input.control"})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := c.Scan(created.Session.ID, created.Session.QR.PID)
	if err != nil {
		t.Fatal(err)
	}
	return pendingFix{coord: c, sid: created.Session.ID, pid: created.Session.QR.PID, pending: scan.PhonePendingToken}
}
