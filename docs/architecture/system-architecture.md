# System architecture

Status: locked baseline for implementation planning. No application code in this milestone.

Legend used throughout the architecture set:

| Label | Meaning |
|---|---|
| **Invariant** | Product rule. Changing it changes what PhoneBeam is. |
| **MVP decision** | Chosen for the first production-shaped path. Can be revisited with an explicit ADR. |
| **Platform limitation** | Android, Play, OEM, or WebRTC constraint. Not something we “fix” in app code. |
| **Security requirement** | Required to keep the consent-first and E2E-media model honest. |
| **Future / non-MVP** | Must not leak into M0–M4 scope. |

## 1. What PhoneBeam is

**Invariant.** PhoneBeam is an attended, consent-first remote Android support product.

A phone owner in any network explicitly approves a short-lived session. An operator then views the Android screen in a PC browser and, only where Android officially allows and the owner granted it, sends supported input. Either side can disconnect. After close or expiry the session cannot silently reopen.

PhoneBeam is not:

- hidden or unattended remote access
- a persistent agent that survives reboot
- an Android security bypass, root requirement, or exploit toolkit
- an MDM / file-transfer / chat / organization suite

If Android does not officially expose a capability, PhoneBeam treats it as unsupported.

## 2. Problems that must stay separate

These are different subsystems. Documents and milestones must not collapse them.

| Concern | Job | Authoritative component |
|---|---|---|
| Pairing | Bind *this* phone to *this* operator session | Public coordinator + owner approval |
| Consent | Human grant of identity, capabilities, and Android system prompts | Phone owner + Android OS |
| Viewing (`screen.read`) | Capture display via MediaProjection; send video | Android capture + WebRTC |
| Control (`input.control`) | Optional AccessibilityService gestures and global actions | Android PEP |
| Signaling | Authenticated SDP/ICE and session control messages | Public coordinator |
| Transport | DTLS-SRTP media and SCTP-over-DTLS DataChannel | Android ↔ browser, optionally via TURN |
| Lifecycle | Create, approve, connect, reconnect, terminate | Coordinator state + Android enforcement |

**Platform limitation.** Remote viewing and remote control are not one Android API. MediaProjection cannot inject input. AccessibilityService is not a screen-capture API for this product.

**MVP decision.** View-only sessions are first-class. Missing Accessibility permission is a valid session, not a failed product path.

## 3. Locked topology

```
 Operator PC
 ┌─────────────────────────────────────────┐
 │  Local Go agent (127.0.0.1)             │
 │   • serve / launch operator UI          │
 │   • localhost bootstrap + local security│
 │   • signaling bridge to coordinator     │
 │   • not the WebRTC media peer           │
 │                                         │
 │  Browser                                │
 │   • RTCPeerConnection (receive media)   │
 │   • DataChannel (send commands, M3+)    │
 └──────────────────┬──────────────────────┘
                    │ TLS (signaling only)
                    v
         ┌──────────────────────┐     short-lived creds
         │ Public Go coordinator│─────────────────┐
         │ session, pairing,    │                 │
         │ authenticated WSS,   │                 v
         │ lifecycle, TURN mint,│         ┌──────────────┐
         │ rate limiting        │         │ STUN / TURN  │
         └──────────┬───────────┘         │ UDP + TCP +  │
                    │                     │ TLS/443      │
                    │ TLS                 └──────┬───────┘
                    v                            │ ICE
         ┌───────────────────────────────────────┴────────┐
         │ Android companion (Kotlin, min API 29)         │
         │  PEP, consent UI, MediaProjection FGS,         │
         │  WebRTC sender, optional AccessibilityService  │
         └────────────────────────────────────────────────┘

 Media plane:  Android  <==== DTLS-SRTP (P2P or TURN) ====>  Browser
 Signal plane: Android  <---- WSS/TLS ----> Coordinator <----> Agent/Browser
```

**MVP decision.** The PC-side WebRTC media peer is the browser `RTCPeerConnection`, not the Go agent and not the coordinator.

**Security requirement.** The public coordinator must never receive screen or audio frames in the normal architecture. TURN relays ciphertext. There is no SFU in the MVP: an SFU would terminate DTLS and see plaintext.

## 4. Component responsibilities

### 4.1 Android companion (Kotlin, min API 29)

**MVP decision.** Minimum API 29 / Android 10.

Responsibilities:

- QR scan against an allowlisted coordinator origin
- Show operator identity, requested capabilities, SAS/PIN
- Record owner grant; probe actual device capabilities
- Enforce every capability locally (PEP)
- MediaProjection capture, foreground service, visible session indicator
- WebRTC send of video when `screen.read` is granted and projection is active
- Optional AccessibilityService for `input.control` (M3+)
- Refuse control into protected/black capture states by default
- Disconnect, revoke, and refuse commands after termination

**Invariant.** Android is the final Policy Enforcement Point. The PC may request capabilities. Android decides whether any command executes.

### 4.2 Operator web UI (React / TypeScript)

- Create session, display QR and SAS/PIN
- Show live session state and granted vs available capabilities
- Receive WebRTC video
- Send versioned control envelopes (M3+)
- Disconnect

The UI is not a permission authority.

### 4.3 Local Go agent

**MVP decision.** Separate from the public coordinator. Not a media peer.

Responsibilities:

- Serve or launch the operator UI where appropriate
- Localhost integration and bootstrap
- Local security boundary (bind loopback, origin checks, unguessable local token — details in later hardening)
- Coordination bridge: authenticated signaling to the public coordinator
- Hold operator-side bootstrap secrets so they are not the long-term browser JS story

The agent does **not**:

- terminate WebRTC media for the MVP
- authorize Android capabilities
- replace the public coordinator
- accept phone connections on the operator LAN as the product path

**Future / non-MVP.** A packaged desktop wrapper (e.g. Wails/Tauri) around the same web UI is compatible with this split. It is not required to start.

### 4.4 Public Go coordinator

One service, not a set of microservices.

- Session creation
- Pairing (short-lived, one-time challenges)
- Authenticated signaling (SDP, ICE, hangup, capability notifications)
- Session lifecycle coordination
- Short-lived TURN credential coordination
- Rate limiting

**Security requirement.** Coordinator logs may contain signaling metadata (including ICE candidates / IPs). That is PII. They must not contain frames, DataChannel command payloads, pairing secrets, or long-lived TURN secrets.

### 4.5 STUN / TURN

**MVP decision.** TURN is part of M2, the first real internet WebRTC milestone, not a later optimization.

Prefer direct ICE (host / server-reflexive) when it works. Treat TURN as the **required fallback**, not an optional extra.

**Platform / network limitation.** China ↔ Bangladesh, mobile CGNAT, UDP blocking, and filtering make TLS/443 TURN a realistic requirement. Direct P2P is not assumed to be the common case on that path.

No custom media relay. No WebSocket-video fallback through the coordinator.

## 5. Capability model

**Invariant.** Session capabilities are explicit. Android enforces them.

Initial conceptual capabilities:

| Capability | Meaning | Early milestone |
|---|---|---|
| `screen.read` | Capture and stream the display | M0 local, M2 remote |
| `input.control` | Supported gestures and navigation | M3+ |
| `audio.read` | Supported non-call audio | M5; not early MVP |

Effective capability for a session is:

```
requested ∩ owner_granted ∩ device_available ∩ not_revoked
```

The PC cannot widen this set. A forged control message that claims `input.control` is dropped if the intersection is empty.

**Platform limitation.** `audio.read` is not “phone audio.” Microphone, app playback capture, and in-call audio are different. **Call audio is out of scope** for this product class.

## 6. Security properties the topology must preserve

**Security requirement.**

- No unauthenticated production or debug remote stream may survive into a product build. Local M0 preview is compile-time / debug-only.
- Pairing completes before any public remote screen streaming (M1 before M2).
- Session close and expiry are terminal. Silent reopen is forbidden.
- Process death, reboot, or lost MediaProjection authorization must never silently restore capture.
- Hard session TTL: **60 minutes** initial architecture default, configurable later.
- Accessibility window-content scraping is out of MVP scope.
- Control into protected/black screens is refused by default.
- No boot receiver, no unattended reconnect, no cached MediaProjection result Intent as a recapture token.

## 7. Milestone mapping to this topology

| Milestone | What this architecture allows | What it still forbids |
|---|---|---|
| M0 | On-device MediaProjection foundation | Networking, QR, control |
| M1 | Coordinator, lifecycle, QR, consent, SAS, grants | Public unauthenticated streaming |
| M2 | Authenticated Android → browser video, ICE/STUN/TURN including TCP/TLS 443 | Control |
| M3 | DataChannel, PEP, optional a11y, tap/swipe/back/home, revocation | Universal HID, a11y scraping |
| M4 | Scroll, supported Unicode text, ICE restart, process-death UX | Immortal sessions |
| M5 | Granular UX, optional supported audio, hardening, OEM matrix | Call audio, SFU, unattended access |

## 8. Related documents

- [Android capability matrix](android-capability-matrix.md)
- [Trust boundaries](trust-boundaries.md)
- [Session lifecycle](session-lifecycle.md)
- [Threat model](threat-model.md)
- [Architecture decisions](architecture-decisions.md)
- [Pairing protocol](../protocol/pairing-protocol.md)
- [Control protocol](../protocol/control-protocol.md)
