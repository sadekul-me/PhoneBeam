package session

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"phonebeam.dev/coordinator/internal/caps"
	"phonebeam.dev/coordinator/internal/ids"
	"phonebeam.dev/coordinator/internal/sas"
)

const ProtocolVersion = 1

var (
	ErrNotFound          = errors.New("session not found")
	ErrExpired           = errors.New("pairing expired")
	ErrReplay            = errors.New("pairing already consumed")
	ErrScanLocked        = errors.New("pairing already scanned")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrUnknownCapability = errors.New("unknown capability")
	ErrUnrequestedGrant  = errors.New("cannot grant unrequested capability")
	ErrTerminal          = errors.New("session is terminal")
	ErrPairingNotPending = errors.New("pairing is not awaiting approval")
	ErrWrongPairing      = errors.New("pairing id mismatch")
	ErrClosed            = errors.New("session is closed")
)

type Role string

const (
	RoleOperator     Role = "operator"
	RolePhone        Role = "phone"
	RolePhonePending Role = "phone_pending"
)

type QRPayload struct {
	V      int    `json:"v"`
	Origin string `json:"origin"`
	SID    string `json:"sid"`
	PID    string `json:"pid"`
	Exp    int64  `json:"exp"`
}

type PublicSession struct {
	ID                    string     `json:"id"`
	State                 State      `json:"state"`
	OperatorDisplayName   string     `json:"operator_display_name"`
	RequestedCapabilities []string   `json:"requested_capabilities"`
	GrantedCapabilities   []string   `json:"granted_capabilities"`
	DeviceCapabilities    []string   `json:"device_capabilities"`
	EffectiveCapabilities []string   `json:"effective_capabilities"`
	SAS                   string     `json:"sas"`
	PairingExpiresAt      time.Time  `json:"pairing_expires_at"`
	SessionExpiresAt      *time.Time `json:"session_expires_at"`
	QR                    *QRPayload `json:"qr,omitempty"`
}

type Session struct {
	ID                  string
	PairingID           string
	State               State
	OperatorDisplayName string
	Requested           []string
	Granted             []string
	Device              []string
	Effective           []string
	SAS                 string
	CreatedAt           time.Time
	PairingExpiresAt    time.Time
	SessionExpiresAt    *time.Time
	OperatorTokenHash   [32]byte
	PhonePendingHash    [32]byte
	PhoneTokenHash      [32]byte
	HasPhonePending     bool
	HasPhoneToken       bool
	ScanLocked          bool
	PairingBurned       bool
}

type Coordinator struct {
	origin      string
	qrTTL       time.Duration
	sessionTTL  time.Duration
	sasKey      []byte
	now         func() time.Time
	mu          sync.Mutex
	byID        map[string]*Session
	pairingToID map[string]string
}

func NewCoordinator(origin string, qrTTL, sessionTTL time.Duration) *Coordinator {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return &Coordinator{
		origin:      origin,
		qrTTL:       qrTTL,
		sessionTTL:  sessionTTL,
		sasKey:      key,
		now:         time.Now,
		byID:        map[string]*Session{},
		pairingToID: map[string]string{},
	}
}

func (c *Coordinator) Origin() string { return c.origin }

type CreateResult struct {
	Session       PublicSession
	OperatorToken string
}

func (c *Coordinator) Create(operatorName string, requested []string) (*CreateResult, error) {
	normalized, ok := caps.Normalize(requested)
	if !ok || len(normalized) == 0 {
		return nil, ErrUnknownCapability
	}
	sid, err := ids.RandomID(18)
	if err != nil {
		return nil, err
	}
	pid, err := ids.RandomID(18)
	if err != nil {
		return nil, err
	}
	opToken, err := ids.RandomID(32)
	if err != nil {
		return nil, err
	}
	if operatorName == "" {
		suffix, err := ids.RandomID(3)
		if err != nil {
			return nil, err
		}
		operatorName = "Support-" + suffix
	}
	now := c.now()
	sess := &Session{
		ID:                  sid,
		PairingID:           pid,
		State:               StateSessionCreated,
		OperatorDisplayName: operatorName,
		Requested:           normalized,
		Granted:             []string{},
		Device:              []string{},
		Effective:           []string{},
		CreatedAt:           now,
		PairingExpiresAt:    now.Add(c.qrTTL),
		OperatorTokenHash:   ids.HashToken(opToken),
	}
	if err := c.apply(sess, EventIssueQR); err != nil {
		return nil, err
	}
	sess.SAS = sas.Display(c.sasKey, c.origin, sid, pid, sess.OperatorDisplayName, normalized)
	c.mu.Lock()
	c.byID[sid] = sess
	c.pairingToID[pid] = sid
	c.mu.Unlock()
	return &CreateResult{Session: c.public(sess, true), OperatorToken: opToken}, nil
}

type ScanResult struct {
	Session           PublicSession
	PhonePendingToken string
}

func (c *Coordinator) Scan(sid, pid string) (*ScanResult, error) {
	pending, err := ids.RandomID(32)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := c.lockedGet(sid)
	if err != nil {
		return nil, err
	}
	c.expireIfNeededLocked(sess)
	if sess.PairingID != pid {
		return nil, ErrWrongPairing
	}
	if sess.State == StateExpired {
		return nil, ErrExpired
	}
	if sess.State.Terminal() {
		return nil, ErrTerminal
	}
	if sess.PairingBurned {
		return nil, ErrReplay
	}
	if sess.ScanLocked || sess.State == StateApprovalPending || sess.State == StatePhoneScanned {
		return nil, ErrScanLocked
	}
	if c.now().After(sess.PairingExpiresAt) {
		return nil, ErrExpired
	}
	if err := c.apply(sess, EventScan); err != nil {
		return nil, err
	}
	if err := c.apply(sess, EventBeginApproval); err != nil {
		return nil, err
	}
	sess.ScanLocked = true
	sess.HasPhonePending = true
	sess.PhonePendingHash = ids.HashToken(pending)
	return &ScanResult{Session: c.public(sess, false), PhonePendingToken: pending}, nil
}

type ApproveInput struct {
	Granted []string
	Device  []string
}

type ApproveResult struct {
	Session    PublicSession
	PhoneToken string
}

func (c *Coordinator) Approve(sid string, token string, in ApproveInput) (*ApproveResult, error) {
	phoneToken, err := ids.RandomID(32)
	if err != nil {
		return nil, err
	}
	granted, ok := caps.Normalize(in.Granted)
	if !ok {
		return nil, ErrUnknownCapability
	}
	device, ok := caps.Normalize(in.Device)
	if !ok {
		return nil, ErrUnknownCapability
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := c.authorizeLocked(sid, token, RolePhonePending)
	if err != nil {
		return nil, err
	}
	c.expireIfNeededLocked(sess)
	if sess.State != StateApprovalPending {
		return nil, ErrPairingNotPending
	}
	if !caps.Subset(granted, sess.Requested) {
		return nil, ErrUnrequestedGrant
	}
	if err := c.apply(sess, EventApprove); err != nil {
		return nil, err
	}
	sess.Granted = granted
	sess.Device = device
	sess.Effective = caps.Intersect(sess.Requested, granted, device)
	if err := c.apply(sess, EventBindCaps); err != nil {
		return nil, err
	}
	now := c.now()
	exp := now.Add(c.sessionTTL)
	sess.SessionExpiresAt = &exp
	sess.PairingBurned = true
	sess.HasPhonePending = false
	sess.HasPhoneToken = true
	sess.PhoneTokenHash = ids.HashToken(phoneToken)
	delete(c.pairingToID, sess.PairingID)
	return &ApproveResult{Session: c.public(sess, false), PhoneToken: phoneToken}, nil
}

func (c *Coordinator) Reject(sid, token string) (*PublicSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := c.authorizeLocked(sid, token, RolePhonePending)
	if err != nil {
		return nil, err
	}
	c.expireIfNeededLocked(sess)
	if sess.State != StateApprovalPending {
		return nil, ErrPairingNotPending
	}
	if err := c.apply(sess, EventReject); err != nil {
		return nil, err
	}
	sess.PairingBurned = true
	sess.HasPhonePending = false
	delete(c.pairingToID, sess.PairingID)
	pub := c.public(sess, false)
	return &pub, nil
}

func (c *Coordinator) Close(sid, token string) (*PublicSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := c.authorizeLocked(sid, token, RoleOperator, RolePhone)
	if err != nil {
		return nil, err
	}
	if sess.State.Terminal() {
		if sess.State == StateClosed {
			return nil, ErrClosed
		}
		return nil, ErrTerminal
	}
	if err := c.apply(sess, EventClose); err != nil {
		return nil, err
	}
	sess.PairingBurned = true
	sess.HasPhonePending = false
	delete(c.pairingToID, sess.PairingID)
	pub := c.public(sess, false)
	return &pub, nil
}

func (c *Coordinator) Get(sid, token string) (*PublicSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := c.authorizeLocked(sid, token, RoleOperator, RolePhone, RolePhonePending)
	if err != nil {
		return nil, err
	}
	c.expireIfNeededLocked(sess)
	includeQR := sess.State == StateQRAvailable && c.roleOf(sess, token) == RoleOperator
	pub := c.public(sess, includeQR)
	return &pub, nil
}

func (c *Coordinator) Sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, sess := range c.byID {
		c.expireIfNeededLocked(sess)
	}
}

func (c *Coordinator) expireIfNeededLocked(sess *Session) {
	now := c.now()
	if !sess.State.Terminal() && sess.SessionExpiresAt != nil && now.After(*sess.SessionExpiresAt) {
		_ = c.apply(sess, EventClose)
		sess.PairingBurned = true
		return
	}
	if (sess.State == StateQRAvailable || sess.State == StatePhoneScanned || sess.State == StateApprovalPending) &&
		now.After(sess.PairingExpiresAt) {
		_ = c.apply(sess, EventExpirePairing)
		sess.PairingBurned = true
		delete(c.pairingToID, sess.PairingID)
	}
}

func (c *Coordinator) apply(sess *Session, event Event) error {
	next, err := Apply(sess.State, event)
	if err != nil {
		return err
	}
	sess.State = next
	return nil
}

func (c *Coordinator) lockedGet(sid string) (*Session, error) {
	sess := c.byID[sid]
	if sess == nil {
		return nil, ErrNotFound
	}
	return sess, nil
}

func (c *Coordinator) authorizeLocked(sid, token string, roles ...Role) (*Session, error) {
	sess, err := c.lockedGet(sid)
	if err != nil {
		return nil, err
	}
	got := c.roleOf(sess, token)
	for _, want := range roles {
		if got == want {
			return sess, nil
		}
	}
	return nil, ErrUnauthorized
}

func (c *Coordinator) roleOf(sess *Session, token string) Role {
	h := ids.HashToken(token)
	if ids.EqualHash(h, sess.OperatorTokenHash) {
		return RoleOperator
	}
	if sess.HasPhonePending && ids.EqualHash(h, sess.PhonePendingHash) {
		return RolePhonePending
	}
	if sess.HasPhoneToken && ids.EqualHash(h, sess.PhoneTokenHash) {
		return RolePhone
	}
	return ""
}

func (c *Coordinator) public(sess *Session, includeQR bool) PublicSession {
	pub := PublicSession{
		ID:                    sess.ID,
		State:                 sess.State,
		OperatorDisplayName:   sess.OperatorDisplayName,
		RequestedCapabilities: append([]string{}, sess.Requested...),
		GrantedCapabilities:   append([]string{}, sess.Granted...),
		DeviceCapabilities:    append([]string{}, sess.Device...),
		EffectiveCapabilities: append([]string{}, sess.Effective...),
		SAS:                   sess.SAS,
		PairingExpiresAt:      sess.PairingExpiresAt,
		SessionExpiresAt:      sess.SessionExpiresAt,
	}
	if includeQR && sess.State == StateQRAvailable && !sess.PairingBurned {
		pub.QR = &QRPayload{
			V:      ProtocolVersion,
			Origin: c.origin,
			SID:    sess.ID,
			PID:    sess.PairingID,
			Exp:    sess.PairingExpiresAt.Unix(),
		}
	}
	return pub
}

func (p QRPayload) Encode() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
