package session

import (
	"encoding/json"
	"time"

	"phonebeam.dev/coordinator/internal/caps"
	"phonebeam.dev/coordinator/internal/ice"
	"phonebeam.dev/coordinator/internal/protocol"
	"phonebeam.dev/coordinator/internal/sas"
)

func sessReachedMedia(sess *Session) bool {
	switch sess.State {
	case StateCapsBound, StateProjectionPending, StateProjectionActive, StateProjectionDenied,
		StateViewNotRequested, StateNegotiating, StateConnected, StateReconnecting,
		StateFailedICE, StateFailedSignaling, StatePeerAuthFailed, StateDisconnecting:
		return true
	default:
		return false
	}
}

func (c *Coordinator) finishCloseLocked(sess *Session) {
	if sess.State.Terminal() {
		return
	}
	if sess.State != StateDisconnecting {
		_ = c.apply(sess, EventClose)
	}
	if sess.State == StateDisconnecting {
		_ = c.apply(sess, EventFinishClose)
	}
	sess.PairingBurned = true
	sess.HasPhonePending = false
	sess.OperatorAttached = false
	sess.PhoneAttached = false
	delete(c.pairingToID, sess.PairingID)
}

func (c *Coordinator) AttachSignal(sid, token string) (Role, *PublicSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := c.authorizeLocked(sid, token, RoleOperator, RolePhone)
	if err != nil {
		return "", nil, err
	}
	c.expireIfNeededLocked(sess)
	if sess.State.Terminal() {
		return "", nil, ErrTerminal
	}
	if !sess.State.AllowsSignaling() && sess.State != StateCapsBound && sess.State != StateProjectionPending && sess.State != StateProjectionDenied {
		return "", nil, ErrSignalingNotAllowed
	}
	role := c.roleOf(sess, token)
	switch role {
	case RoleOperator:
		sess.OperatorAttached = true
	case RolePhone:
		sess.PhoneAttached = true
	default:
		return "", nil, ErrUnauthorized
	}
	pub := c.public(sess, false, sessReachedMedia(sess))
	return role, &pub, nil
}

func (c *Coordinator) DetachSignal(sid string, role Role) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess := c.byID[sid]
	if sess == nil {
		return
	}
	if role == RoleOperator {
		sess.OperatorAttached = false
	}
	if role == RolePhone {
		sess.PhoneAttached = false
	}
}

func (c *Coordinator) RoleAttached(sid string, role Role) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess := c.byID[sid]
	if sess == nil {
		return false
	}
	if role == RoleOperator {
		return sess.OperatorAttached
	}
	return sess.PhoneAttached
}

func (c *Coordinator) ICEConfig(sid, token string) (*ice.Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := c.authorizeLocked(sid, token, RoleOperator, RolePhone)
	if err != nil {
		return nil, err
	}
	c.expireIfNeededLocked(sess)
	if sess.State.Terminal() {
		return nil, ErrTerminal
	}
	built := ice.Build(sess.ID, ice.Options{
		STUNURIs:        c.ice.STUNURIs,
		TURNURIs:        c.ice.TURNURIs,
		TURNSecret:      c.ice.TURNSecret,
		TTL:             10 * time.Minute,
		TransportPolicy: c.ice.TransportPolicy,
		Now:             c.now(),
	})
	return &built, nil
}

func (c *Coordinator) HandleSignal(sid string, role Role, env *protocol.Envelope) (*PublicSession, *protocol.Envelope, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess := c.byID[sid]
	if sess == nil {
		return nil, nil, ErrNotFound
	}
	c.expireIfNeededLocked(sess)
	if sess.State.Terminal() {
		return nil, nil, ErrTerminal
	}
	if err := protocol.Validate(env, string(role), string(sess.State), true); err != nil {
		return nil, nil, err
	}
	switch env.Type {
	case "projection":
		if err := c.applyProjectionLocked(sess, env.Projection); err != nil {
			return nil, nil, err
		}
	case "sdp_offer":
		if !caps.Contains(sess.Effective, caps.ScreenRead) {
			return nil, nil, ErrMissingScreenRead
		}
		if sess.State == StateProjectionActive {
			if err := c.apply(sess, EventBeginNegotiate); err != nil {
				return nil, nil, err
			}
		}
		sess.OfferSeen = true
	case "sdp_answer":
		if sess.State != StateNegotiating && sess.State != StateReconnecting {
			return nil, nil, ErrSignalingNotAllowed
		}
		sess.AnswerSeen = true
	case "need_offer":
		if role != RoleOperator {
			return nil, nil, protocol.ErrNotAllowed
		}
		if sess.RehandshakeUsed {
			c.finishCloseLocked(sess)
			pub := c.public(sess, false, false)
			return &pub, nil, nil
		}
		sess.RehandshakeUsed = true
		sess.OfferSeen = false
		sess.AnswerSeen = false
		sess.PhoneReady = false
		sess.OperatorReady = false
		if sess.State == StateConnected || sess.State == StateFailedICE {
			_ = c.apply(sess, EventIceRestart)
		}
	case "peer_fingerprint":
		fp := sas.NormalizeFingerprint(env.Fingerprint.Value)
		if role == RolePhone {
			sess.AndroidFP = fp
		} else {
			sess.BrowserFP = fp
		}
	case "peer_ready":
		if role == RolePhone {
			sess.PhoneReady = true
		} else {
			sess.OperatorReady = true
		}
		if err := c.tryConnectLocked(sess); err != nil {
			return nil, nil, err
		}
	case "stats":
		if env.Stats != nil && env.Stats.Path != "" {
			sess.ConnectionPath = env.Stats.Path
		}
		if env.Stats != nil && env.Stats.Width > 0 {
			sess.CaptureWidth = env.Stats.Width
			sess.CaptureHeight = env.Stats.Height
		}
	case "hangup":
		c.finishCloseLocked(sess)
	case "failed_ice":
		_ = c.apply(sess, EventFailedICE)
	case "failed_signaling":
		_ = c.apply(sess, EventFailedSignaling)
	}
	forward := stripToken(env)
	pub := c.public(sess, false, sessReachedMedia(sess))
	return &pub, forward, nil
}

func (c *Coordinator) applyProjectionLocked(sess *Session, p *protocol.Projection) error {
	if p == nil {
		return protocol.ErrMalformed
	}
	if p.Width > 0 {
		sess.CaptureWidth = p.Width
		sess.CaptureHeight = p.Height
	}
	if p.Scope != "" {
		sess.CaptureScope = p.Scope
	}
	switch p.Status {
	case "pending":
		if sess.State == StateCapsBound || sess.State == StateProjectionDenied {
			return c.apply(sess, EventProjectionPending)
		}
		if sess.State == StateProjectionPending {
			return nil
		}
		return &InvalidTransitionError{From: sess.State, Event: EventProjectionPending}
	case "active":
		sess.ProjectionLive = true
		if sess.State == StateCapsBound {
			if err := c.apply(sess, EventProjectionPending); err != nil {
				return err
			}
		}
		if sess.State == StateProjectionPending {
			return c.apply(sess, EventProjectionActive)
		}
		return nil
	case "denied":
		sess.ProjectionLive = false
		if sess.State == StateCapsBound {
			if err := c.apply(sess, EventProjectionPending); err != nil {
				return err
			}
		}
		if sess.State == StateProjectionPending {
			return c.apply(sess, EventProjectionDenied)
		}
		if sess.State.AllowsSignaling() || sess.State == StateConnected {
			c.finishCloseLocked(sess)
			return nil
		}
		return nil
	default:
		return protocol.ErrMalformed
	}
}

func (c *Coordinator) tryConnectLocked(sess *Session) error {
	if sess.State != StateNegotiating && sess.State != StateReconnecting {
		return nil
	}
	if !caps.Contains(sess.Effective, caps.ScreenRead) {
		return ErrMissingScreenRead
	}
	if !sess.ProjectionLive || !sess.OfferSeen || !sess.AnswerSeen || !sess.PhoneReady || !sess.OperatorReady {
		return nil
	}
	if sess.AndroidFP == "" || sess.BrowserFP == "" {
		return nil
	}
	if err := c.apply(sess, EventConnected); err != nil {
		return err
	}
	return nil
}

func (c *Coordinator) SetProjection(sid, token string, p *protocol.Projection) (*PublicSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sess, err := c.authorizeLocked(sid, token, RolePhone)
	if err != nil {
		return nil, err
	}
	if err := c.applyProjectionLocked(sess, p); err != nil {
		return nil, err
	}
	pub := c.public(sess, false, sessReachedMedia(sess))
	return &pub, nil
}

func stripToken(env *protocol.Envelope) *protocol.Envelope {
	if env == nil {
		return nil
	}
	clone := *env
	clone.Token = ""
	return &clone
}

func MarshalEnvelope(env *protocol.Envelope) ([]byte, error) {
	return json.Marshal(env)
}
