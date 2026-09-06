# Session lifecycle

Legend: **Invariant** · **MVP decision** · **Platform limitation** · **Security requirement** · **Future / non-MVP**

## 1. Design goals

- Every remote session is created, paired, approved, connected, and **really** terminated.
- Pairing and MediaProjection consent are different steps.
- Reconnect cannot skip consent or revive a dead projection.
- Closed and expired sessions cannot silently reopen.

**MVP decision.** Hard session maximum TTL is **60 minutes** from creation (or from approval — exact anchor is an open detail; see §7). Configurable later. Support sessions are not immortal.

## 2. State machine

```
 IDLE
   │ operator creates session
   v
 SESSION_CREATED
   │ coordinator mints pairing challenge
   v
 QR_AVAILABLE ── TTL elapsed ──────────────────────────────► EXPIRED
   │ phone authenticates challenge
   v
 PHONE_SCANNED ── second phone / consumed challenge ───────► REJECTED_CONSUMED
   │ consent UI shown
   v
 APPROVAL_PENDING ── owner deny / owner timeout ───────────► REJECTED
   │ owner accept (pairing burned)
   v
 APPROVED
   │ caps = requested ∩ granted ∩ device probe
   v
 CAPS_BOUND
   │ if screen.read: system MediaProjection dialog
   v
 PROJECTION_ACTIVE  or  PROJECTION_DENIED  or  VIEW_NOT_REQUESTED
   │ authenticated SDP / ICE  (M2+)
   v
 NEGOTIATING ── ICE exhausted ──► FAILED_ICE ──► DISCONNECTING
   │ DTLS bound; first authorized frame policy met
   v
 CONNECTED ◄──── ICE restart ──── RECONNECTING
   │
   ├── either peer disconnects
   ├── projection stop / lock / process death / reboot
   ├── capability-only shutdown
   └── session max TTL
   v
 DISCONNECTING
   v
 CLOSED
```

`VIEW_NOT_REQUESTED` exists so a future caps-only session is representable. Early MVP always requests `screen.read` for a useful support call, but the machine must not assume capture.

## 3. State meanings

| State | Meaning |
|---|---|
| `IDLE` | No session object |
| `SESSION_CREATED` | Coordinator has a `sid`; no usable QR yet or QR not issued |
| `QR_AVAILABLE` | Short-lived one-time challenge is displayable |
| `PHONE_SCANNED` | A phone presented a valid unused challenge |
| `APPROVAL_PENDING` | Owner sees identity, requested caps, SAS/PIN |
| `APPROVED` | Owner accepted; pairing challenge is burned |
| `CAPS_BOUND` | Effective capability set frozen except for **narrowing** (revocation) |
| `PROJECTION_ACTIVE` | MediaProjection FGS running with current system consent |
| `PROJECTION_DENIED` | Owner approved PhoneBeam but refused or lost system capture |
| `NEGOTIATING` | Authenticated WebRTC signaling in progress |
| `CONNECTED` | Authorized media and/or control channel per grants |
| `RECONNECTING` | ICE restart on the **same** projection and pairing grant |
| `DISCONNECTING` | Tokens being revoked, PEP closing |
| `CLOSED` | Terminal success/failure hangup |
| `EXPIRED` | Terminal; QR or session TTL |
| `REJECTED` | Owner refused |
| `REJECTED_CONSUMED` | Challenge no longer usable (race or replay) |
| `FAILED_ICE` | Connectivity failed after retries |

Terminal: `CLOSED`, `EXPIRED`, `REJECTED`, `REJECTED_CONSUMED`. `FAILED_ICE` always proceeds to `DISCONNECTING` → `CLOSED`.

## 4. Invalid transitions

**Security requirement.** The coordinator and both clients must reject these.

| From | Illegal destination | Why |
|---|---|---|
| `IDLE` | `CONNECTED` / `APPROVED` | Skip pairing |
| `QR_AVAILABLE` | `CONNECTED` | Skip consent |
| `PHONE_SCANNED` | `CONNECTED` | Skip owner approval |
| `APPROVAL_PENDING` | `QR_AVAILABLE` | Would resurrect the same challenge |
| `CLOSED` / `EXPIRED` / `REJECTED*` | any active state | Silent reopen |
| `CONNECTED` | `QR_AVAILABLE` | Same QR reconnect |
| any | `CONNECTED` with `screen.read` but without `PROJECTION_ACTIVE` | Capture without system consent |
| `RECONNECTING` | `PROJECTION_ACTIVE` via cached token after `onStop` | Silent recapture |

## 5. Reconnect vs recapture

**MVP decision (M4 rules, designed now).**

Allowed: `CONNECTED` → `RECONNECTING` → `CONNECTED`

- Same `sid`
- Same pairing grant
- Same MediaProjection session still alive
- ICE restart / network switch (Wi-Fi ↔ cellular)
- Operator WebRTC re-handshake **only** while Android projection remains alive (tab refresh policy: see unresolved list)

Forbidden silent restore:

- Process death
- Reboot
- Screen lock that stopped projection
- User stopped capture from the notification
- `MediaProjection` token reuse after `onStop`
- Any path that skips the system capture dialog

**Platform limitation.** Android 14+ throws if the capture token is reused. PhoneBeam must not try.

**Invariant.** Reboot never resumes a session. No boot persistence.

## 6. Capability changes during a live session

Caps may **narrow**, never widen, without a new pairing.

| Event | Result |
|---|---|
| Owner revokes `input.control` | PEP drops control immediately; `capability_update` |
| AccessibilityService disabled | `input.control` unavailable |
| Owner stops projection | Viewing ends; typically `DISCONNECTING` if `screen.read` was the session’s purpose |
| M5 granular UX toggles | Still PEP-enforced on device |

## 7. Timeouts

| Timer | Applies | On fire |
|---|---|---|
| Pairing QR TTL | `QR_AVAILABLE` … `APPROVAL_PENDING` | `EXPIRED` |
| Owner approval timeout | `APPROVAL_PENDING` | `REJECTED` or `EXPIRED` |
| ICE timeout | `NEGOTIATING` / `RECONNECTING` | `FAILED_ICE` |
| Session max TTL (60 min default) | Any post-create active state | `DISCONNECTING` → `CLOSED` |

Exact QR TTL seconds and whether session TTL starts at create vs approve are **unresolved** (see [architecture decisions](architecture-decisions.md)). The existence of both timers is locked.

## 8. Milestone coverage

| Milestone | Lifecycle scope |
|---|---|
| M0 | Projection start/stop, rotation, lock, FGS — no session machine on the network |
| M1 | `IDLE` … `CAPS_BOUND` / reject / expire (no public media) |
| M2 | `NEGOTIATING` … `CONNECTED` viewing |
| M3 | Control on `CONNECTED`; revocation |
| M4 | `RECONNECTING`, process-death UX, capability_update |
| M5 | Rate limits, production timeout tuning |
