package session

// Coordinator session states. CONNECTED is only reachable after M2 media conditions.
type State string

const (
	StateSessionCreated    State = "SESSION_CREATED"
	StateQRAvailable       State = "QR_AVAILABLE"
	StatePhoneScanned      State = "PHONE_SCANNED"
	StateApprovalPending   State = "APPROVAL_PENDING"
	StateApproved          State = "APPROVED"
	StateCapsBound         State = "CAPS_BOUND"
	StateProjectionPending State = "PROJECTION_PENDING"
	StateProjectionActive  State = "PROJECTION_ACTIVE"
	StateProjectionDenied  State = "PROJECTION_DENIED"
	StateViewNotRequested  State = "VIEW_NOT_REQUESTED"
	StateNegotiating       State = "NEGOTIATING"
	StateConnected         State = "CONNECTED"
	StateReconnecting      State = "RECONNECTING"
	StateFailedICE         State = "FAILED_ICE"
	StateFailedSignaling   State = "FAILED_SIGNALING"
	StatePeerAuthFailed    State = "PEER_AUTH_FAILED"
	StateDisconnecting     State = "DISCONNECTING"
	StateRejected          State = "REJECTED"
	StateRejectedConsumed  State = "REJECTED_CONSUMED" // scanner-local; session stays with the first lock holder
	StateExpired           State = "EXPIRED"
	StateClosed            State = "CLOSED"
)

func (s State) Terminal() bool {
	switch s {
	case StateRejected, StateRejectedConsumed, StateExpired, StateClosed:
		return true
	default:
		return false
	}
}

func (s State) AllowsSignaling() bool {
	switch s {
	case StateProjectionActive, StateNegotiating, StateConnected, StateReconnecting, StateFailedICE:
		return true
	default:
		return false
	}
}

type Event string

const (
	EventCreated           Event = "created"
	EventIssueQR           Event = "issue_qr"
	EventScan              Event = "scan"
	EventBeginApproval     Event = "begin_approval"
	EventApprove           Event = "approve"
	EventBindCaps          Event = "bind_caps"
	EventReject            Event = "reject"
	EventExpirePairing     Event = "expire_pairing"
	EventProjectionPending Event = "projection_pending"
	EventProjectionActive  Event = "projection_active"
	EventProjectionDenied  Event = "projection_denied"
	EventViewNotRequested  Event = "view_not_requested"
	EventBeginNegotiate    Event = "begin_negotiate"
	EventConnected         Event = "connected"
	EventIceRestart        Event = "ice_restart"
	EventFailedICE         Event = "failed_ice"
	EventFailedSignaling   Event = "failed_signaling"
	EventPeerAuthFailed    Event = "peer_auth_failed"
	EventClose             Event = "close"
	EventFinishClose       Event = "finish_close"
)

// Apply is a pure transition table. HTTP/signaling handlers must not invent extra edges.
func Apply(from State, event Event) (State, error) {
	next, ok := transitions[from][event]
	if !ok {
		return from, &InvalidTransitionError{From: from, Event: event}
	}
	return next, nil
}

var transitions = map[State]map[Event]State{
	StateSessionCreated: {
		EventIssueQR: StateQRAvailable,
		EventClose:   StateDisconnecting,
	},
	StateQRAvailable: {
		EventScan:          StatePhoneScanned,
		EventExpirePairing: StateExpired,
		EventClose:         StateDisconnecting,
	},
	StatePhoneScanned: {
		EventBeginApproval: StateApprovalPending,
		EventExpirePairing: StateExpired,
		EventClose:         StateDisconnecting,
	},
	StateApprovalPending: {
		EventApprove:       StateApproved,
		EventReject:        StateRejected,
		EventExpirePairing: StateExpired,
		EventClose:         StateDisconnecting,
	},
	StateApproved: {
		EventBindCaps: StateCapsBound,
		EventClose:    StateDisconnecting,
	},
	StateCapsBound: {
		EventProjectionPending: StateProjectionPending,
		EventViewNotRequested:  StateViewNotRequested,
		EventClose:             StateDisconnecting,
	},
	StateProjectionPending: {
		EventProjectionActive: StateProjectionActive,
		EventProjectionDenied: StateProjectionDenied,
		EventClose:            StateDisconnecting,
	},
	StateProjectionActive: {
		EventBeginNegotiate:   StateNegotiating,
		EventProjectionDenied: StateDisconnecting,
		EventClose:            StateDisconnecting,
	},
	StateProjectionDenied: {
		EventProjectionPending: StateProjectionPending,
		EventClose:             StateDisconnecting,
	},
	StateViewNotRequested: {
		EventClose: StateDisconnecting,
	},
	StateNegotiating: {
		EventConnected:        StateConnected,
		EventFailedICE:        StateFailedICE,
		EventFailedSignaling:  StateFailedSignaling,
		EventPeerAuthFailed:   StatePeerAuthFailed,
		EventProjectionDenied: StateDisconnecting,
		EventClose:            StateDisconnecting,
	},
	StateConnected: {
		EventIceRestart:       StateReconnecting,
		EventFailedICE:        StateFailedICE,
		EventProjectionDenied: StateDisconnecting,
		EventClose:            StateDisconnecting,
	},
	StateReconnecting: {
		EventConnected:        StateConnected,
		EventFailedICE:        StateFailedICE,
		EventProjectionDenied: StateDisconnecting,
		EventClose:            StateDisconnecting,
	},
	StateFailedICE: {
		EventIceRestart: StateReconnecting,
		EventClose:      StateDisconnecting,
	},
	StateFailedSignaling: {
		EventClose: StateDisconnecting,
	},
	StatePeerAuthFailed: {
		EventClose: StateDisconnecting,
	},
	StateDisconnecting: {
		EventFinishClose: StateClosed,
	},
}

type InvalidTransitionError struct {
	From  State
	Event Event
}

func (e *InvalidTransitionError) Error() string {
	return "invalid transition from " + string(e.From) + " via " + string(e.Event)
}
