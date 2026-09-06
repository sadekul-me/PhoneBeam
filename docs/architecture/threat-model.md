# Threat model

Legend: **Invariant** · **MVP decision** · **Platform limitation** · **Security requirement** · **Future / non-MVP**

PhoneBeam’s valuable assets are the phone’s screen, the owner’s attention/trust, session tokens, and TURN bandwidth. The product is attended support; the model assumes a human owner can refuse, compare a SAS/PIN, and disconnect. It does not assume a globally trusted coordinator.

## 1. Actors

| Actor | Intent |
|---|---|
| Phone owner | Legitimate control of their device |
| Support operator | Help, within granted caps |
| Malicious operator / compromised PC | Over-collect, send unauthorized commands |
| Network attacker | Tamper, block, replay |
| Compromised coordinator | Rewrite signaling, mint sessions, MITM WebRTC |
| Malicious TURN | Relay abuse, traffic analysis |
| Local malware on operator PC | Drive localhost dashboard |
| Random internet | Scan QR photos, hit APIs, exhaust TURN |

## 2. Threats and mitigations

### Stolen QR screenshot

**Impact.** Stranger pairs later.

**Mitigation (security requirement).** Short TTL, one-time consume, session-bound challenge, SAS/PIN mismatch. A photograph of an old QR must be useless.

### QR race / shoulder surfing

**Impact.** Wrong phone or attacker phone reaches `APPROVAL_PENDING` first.

**Mitigation.** Exclusive lock from scan; second scanner → `REJECTED_CONSUMED`. Owner compares SAS/PIN with the operator over WhatsApp/voice. Operator display name shown on the phone.

### Replay of pairing or tokens

**Impact.** Join or resurrect a session.

**Mitigation.** Consume-on-accept; burned QR; role-bound short-lived tokens; `CLOSED`/`EXPIRED` cannot re-enter; new `sid` for every support call.

### Session hijack (stolen WSS token)

**Impact.** Watch or control a live session.

**Mitigation.** Short-lived tokens issued after accept; bind to `sid` + role (`phone` vs `operator`); revoke on close; do not allow a second operator identity to attach to a live `sid` (MVP).

### Compromised signaling server / SDP rewrite

**Impact.** Attacker terminates DTLS on both sides and **sees plaintext frames**. TLS to the coordinator does not stop this.

**Mitigation (security requirement).** SAS/PIN on both endpoints; bind authenticated WebRTC peer identity / DTLS fingerprints before treating media as `CONNECTED`. Allowlisted coordinator origins on Android. No SFU.

This is the central reason pairing is not “a disposable room code.”

### Malicious TURN server

**Impact.** Bandwidth theft; metadata; **not** frames if DTLS fingerprints are authentic.

**Mitigation.** Short-lived session-scoped TURN credentials from the coordinator; quota/rate limits; still require fingerprint binding. Do not put the long-term TURN secret in the Android app or web bundle.

### Malicious PC / operator XSS / localhost CSRF

**Impact.** Commands sent without the intended human; session created without operator intent.

**Mitigation.** Android PEP on every command. Local agent binds loopback, origin checks, local token (M5 hardening designed now). Browser is not the PEP.

### Unauthorized control command

**Impact.** Touch or navigation without grant.

**Mitigation (invariant).** Android drops commands outside the effective capability set. Rate-limit gestures. Unknown protocol versions fail closed.

### Token / secret leakage in logs

**Impact.** Hijack, pairing steal, TURN theft.

**Mitigation.** No frames, no command bodies, no pairing secrets, no raw tokens in logs. SDP/ICE dumps are PII and debug-only.

### Protocol downgrade

**Impact.** Skip SAS, skip fingerprint bind, inject an extra media peer, force a plaintext path.

**Mitigation.** Minimum protocol version; ICE servers only from coordinator credential response; ignore peer-supplied extra ICE servers; refuse a third media endpoint; no WebSocket-video fallback.

### Stale session reuse / silent recapture

**Impact.** Capture after the owner thinks the session ended.

**Mitigation (invariant).** Terminal `CLOSED`/`EXPIRED`. Process death, reboot, lock-stop, and lost MediaProjection never silently restore capture. No boot receiver. No cached projection Intent.

### Fake device identity

**Impact.** Operator believes they are on the customer’s phone.

**Mitigation.** Attended product: SAS comparison plus out-of-band voice/chat. Show coarse device model. Do not pretend cryptographic device attestation is complete in MVP (Play Integrity is **future / non-MVP**).

### DoS / rate abuse

**Impact.** Pairing exhaustion, TURN bills, signaling overload.

**Mitigation.** Coordinator rate limits (designed in M1, production-hardened in M5). Per-IP and per-operator create/scan limits. TURN allocation quota per `sid`. Session max TTL 60 minutes.

### Sensitive content in recordings

**Impact.** Support footage becomes a breach.

**Mitigation.** **No screen recording by default.** Audit events only: create, scan, grant, connect, cap change, close.

### Accessibility over-privilege

**Impact.** Play rejection; product looks like a RAT.

**Mitigation.** Gestures and global actions only. No window-content scraping in MVP. No overlay click-through. No auto-confirm of permission dialogs. Refuse inject into black/protected capture.

### Phishing QR (evil coordinator URL)

**Impact.** Phone sends pairing to an attacker.

**Mitigation.** Android allowlist of coordinator origins. Do not follow arbitrary QR URLs.

### Social engineering

**Impact.** Owner approves the wrong person.

**Mitigation.** UI copy must state that a remote human will see the screen **now**. Large disconnect. Session TTL. This is not fully solvable in protocol.

## 3. Explicit non-features that would worsen the model

**Invariant.** Do not add:

- unattended access
- boot persistence
- silent capture
- PC-authoritative permissions
- SFU / coordinator media
- unauthenticated debug streams in product builds
- a11y tree harvesting to beat `FLAG_SECURE`

## 4. Residual risk (accepted)

| Risk | Why accepted |
|---|---|
| Owner approves a real attacker | Attended consent cannot stop a deceived owner |
| Compromised Android OS | Below the PEP |
| Traffic analysis on TURN | Relay sees sizes/timing |
| OEM kills FGS | Reliability, not a bypass |
| View-only cannot help some issues | Honesty over fake control |

## 5. Milestone relevance

M1 must make stolen/replayed QR fail.
M2 must make coordinator-MITM of media fail closed if fingerprints do not bind.
M3 must make unauthorized commands fail on device.
M5 hardens rate limits, logs, and the localhost agent.
