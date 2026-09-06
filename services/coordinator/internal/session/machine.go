package session

// M1 coordinator states. CONNECTED and media states are M2 and are rejected here.
type State string

const (
	StateSessionCreated   State = "SESSION_CREATED"
	StateQRAvailable      State = "QR_AVAILABLE"
	StatePhoneScanned     State = "PHONE_SCANNED"
	StateApprovalPending  State = "APPROVAL_PENDING"
	StateApproved         State = "APPROVED"
	StateCapsBound        State = "CAPS_BOUND"
	StateRejected         State = "REJECTED"
	StateRejectedConsumed State = "REJECTED_CONSUMED" // scanner-local; session stays with the first lock holder
	StateExpired          State = "EXPIRED"
	StateClosed           State = "CLOSED"
)

func (s State) Terminal() bool {
	switch s {
	case StateRejected, StateRejectedConsumed, StateExpired, StateClosed:
		return true
	default:
		return false
	}
}

type Event string

const (
	EventCreated       Event = "created"
	EventIssueQR       Event = "issue_qr"
	EventScan          Event = "scan"
	EventBeginApproval Event = "begin_approval"
	EventApprove       Event = "approve"
	EventBindCaps      Event = "bind_caps"
	EventReject        Event = "reject"
	EventExpirePairing Event = "expire_pairing"
	EventClose         Event = "close"
)

// Apply is a pure transition table. HTTP handlers must not invent extra edges.
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
		EventClose:   StateClosed,
	},
	StateQRAvailable: {
		EventScan:          StatePhoneScanned,
		EventExpirePairing: StateExpired,
		EventClose:         StateClosed,
	},
	StatePhoneScanned: {
		EventBeginApproval: StateApprovalPending,
		EventExpirePairing: StateExpired,
		EventClose:         StateClosed,
	},
	StateApprovalPending: {
		EventApprove:       StateApproved,
		EventReject:        StateRejected,
		EventExpirePairing: StateExpired,
		EventClose:         StateClosed,
	},
	StateApproved: {
		EventBindCaps: StateCapsBound,
		EventClose:    StateClosed,
	},
	StateCapsBound: {
		EventClose: StateClosed,
	},
}

type InvalidTransitionError struct {
	From  State
	Event Event
}

func (e *InvalidTransitionError) Error() string {
	return "invalid transition from " + string(e.From) + " via " + string(e.Event)
}
