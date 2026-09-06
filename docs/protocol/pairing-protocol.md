# Pairing protocol (conceptual)

Status: architecture for M1. No wire-format implementation in this milestone.

Legend: **Invariant** · **MVP decision** · **Platform limitation** · **Security requirement** · **Future / non-MVP**

## 1. Goals

**Security requirement.** Pair a specific Android device to a specific operator session such that:

- the QR contains no permanent password
- the challenge is short-lived and one-time
- replay of screenshots fails
- the owner explicitly approves requested capabilities
- the challenge is bound to one `sid`
- Android only talks to an allowlisted coordinator origin
- a later SAS/PIN binds human-visible identity, and before media, WebRTC DTLS fingerprints

**Invariant.** No pairing, no public remote stream. M0 local preview is not this protocol.

## 2. Roles

| Role | Job |
|---|---|
| Operator UI + local agent | Create session, display QR and SAS, bridge signaling |
| Public coordinator | Mint/consume challenges, authenticate signaling, mint TURN creds later |
| Android app | Scan, origin allowlist, consent UI, PEP, later WebRTC |
| Phone owner | Compare SAS, grant or refuse caps |

The coordinator is **not** the PEP and **not** a media peer.

## 3. QR contents

The QR is a routing handle, not a credential vault.

Conceptual fields:

```
v              protocol version
origin         coordinator origin (must match Android allowlist)
sid            session id
pid            pairing challenge id
```

**Must not appear in the QR:** long-term passwords, TURN static secret, operator private keys, reusable room codes.

The high-entropy pairing secret stays server-side (or a verifier is single-use). Knowing a photographed QR after consume/expiry is useless.

**Security requirement.** If `origin` is not allowlisted, the Android app refuses to contact it. Arbitrary HTTPS in a QR is phishing.

## 4. Message sequence (M1)

```
Operator                Coordinator                 Android                 Owner
   |                         |                         |                      |
   | create_session          |                         |                      |
   | requested_caps          |                         |                      |
   |------------------------>|                         |                      |
   |                         | SESSION_CREATED         |                      |
   |                         | mint pid, TTL           |                      |
   | QR_AVAILABLE            |                         |                      |
   |<------------------------|                         |                      |
   | show QR + SAS_A         |                         |                      |
   |                         |                         | scan QR              |
   |                         |                         | allowlist origin     |
   |                         | PHONE_SCANNED           |                      |
   |                         | exclusive lock          |                      |
   |                         |<------------------------|                      |
   |                         | operator identity       |                      |
   |                         | requested_caps          |                      |
   |                         | SAS material            |                      |
   |                         |------------------------>|                      |
   |                         | APPROVAL_PENDING        | show identity,       |
   |                         |                         | caps, SAS_B          |
   |                         |                         |--------------------->|
   |                         |                         | accept / reject      |
   |                         |                         |<---------------------|
   |                         | APPROVED (burn pid)     |                      |
   |                         | or REJECTED             |                      |
   |                         |<------------------------|                      |
   |                         | session tokens          |                      |
   |                         | CAPS_BOUND              |                      |
   |<------------------------|------------------------>|                      |
```

Public video streaming is **not** part of this sequence. That is M2, using the tokens issued here.

## 5. Consume, expiry, replay

**Recommended default (open decision O1–O3, implement as specified unless superseded):**

| Rule | Behavior |
|---|---|
| TTL | Challenge dies in `QR_AVAILABLE` / `APPROVAL_PENDING` (60–120s recommended) |
| Exclusive lock | First valid scan holds `PHONE_SCANNED`; a second phone gets `REJECTED_CONSUMED` |
| Burn | Accept burns `pid`. Reject or timeout burns `pid`. The QR never returns to `QR_AVAILABLE` |
| Session bind | `pid` works only for its `sid` |
| Replay | Presenting a burned or expired `pid` fails closed |

**MVP decision.** Creating a new session mints a **new** `sid` and `pid`. There is no “same QR reconnect.”

## 6. Consent and capabilities

At `APPROVAL_PENDING` the owner sees:

- operator display name / ephemeral operator session identity
- requested capabilities (`screen.read`, `input.control`, `audio.read` as applicable)
- SAS/PIN
- that a remote human will see the screen if they approve `screen.read`

Owner accept writes the granted set. Android intersects with device availability (`CAPS_BOUND`).

**Invariant.** The PC cannot later assert a wider set. Widening requires a new pairing.

`audio.read` may appear in the model; early milestones must not request it.

## 7. SAS/PIN and DTLS fingerprint binding

**Security requirement.** Coordinator TLS authenticates the *channel to the coordinator*, not the *WebRTC peer*. A compromised coordinator can swap SDP fingerprints and sit in the middle of media.

Two-phase identity:

### Phase A — pairing SAS (M1)

Both screens show a short PIN derived from the pairing transcript (version, origin, `sid`, `pid`, operator identity, requested caps, coordinator-held nonce).

The owner compares this with the operator (WhatsApp/voice). Mismatch → reject.

Exact KDF is **unresolved (O4/O5)**. Architecture requirement: unguessable, not the QR text itself, displayed in grouped digits or similar.

### Phase B — WebRTC peer bind (M2, before `CONNECTED`)

After SDP exchange on the **already authenticated** signaling channel:

- each side records the remote DTLS fingerprint
- fingerprints are bound into the session transcript
- media is not shown as connected, and Android must not send frames as a successful support session, if the bind fails
- both UIs can show an updated SAS that covers fingerprints, or verify silently **in addition to** Phase A — silent-only verification does **not** defend against a compromised coordinator

**MVP decision.** Phase B is part of M2, not a later hardening nice-to-have. M1 still ships Phase A so pairing is not a bare room code.

## 8. Tokens after approval

Conceptual, not a JWT spec:

| Token | Held by | Use |
|---|---|---|
| Operator session token | Agent / browser via agent | Signaling as `role=operator` |
| Phone session token | Android | Signaling as `role=phone` |
| TURN REST credentials | Both, minted at M2 connect | ICE only, short TTL, scoped to `sid` |

Tokens die on `CLOSED` / `EXPIRED` / TTL. They are not in the QR.

## 9. What this protocol explicitly does not do

- Unattended reconnect with a stored password
- Pairing that implies MediaProjection (system dialog remains separate)
- Pairing that implies Accessibility enabled
- Following non-allowlisted coordinator URLs
- Shipping debug “open room” streams in product builds
