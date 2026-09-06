# Trust boundaries

Legend: **Invariant** · **MVP decision** · **Platform limitation** · **Security requirement** · **Future / non-MVP**

## 1. Boundary map

```
 [Phone owner]  -- human consent -->  [PhoneBeam Android app]  -- public APIs -->  [Android OS]
                                            |                                        ^
                                            | DTLS-SRTP media                         |
                                            v                                        |
 [Operator] --> [Browser] <--> [Local Go agent]                                      |
                     \              |  signaling bridge                              |
                      \             v                                                |
                       --> [Public coordinator]                                      |
                                    |                                                |
                                    v                                                |
                             [STUN / TURN] ------------------------------------------+
                                    |
                                    v
                           [Network / ISP / filtering]
```

Each box is a different trust zone. Compromising one must not automatically grant the powers of another.

## 2. Zones

| Zone | Trust stance | Can see | Must not |
|---|---|---|---|
| Phone owner + Android OS | Highest authority on the device | Everything on the device | Be bypassed |
| PhoneBeam Android app | **PEP** | Captured frames, grants, session keys | Start without explicit UI consent; persist after close; execute denied caps |
| Operator browser | Untrusted input + media sink | Decrypted media after DTLS | Authorize capabilities; widen grants |
| Local Go agent | Operator-PC helper | Operator bootstrap tokens, signaling | Media plaintext (MVP); Android permission authority; public pairing authority |
| Public coordinator | Untrusted for media and PEP | Pairing metadata, SDP, ICE, session state | Frames, audio, DataChannel bodies, long-lived pairing secrets |
| TURN | Untrusted relay | DTLS/SRTP ciphertext, timing, volume | Plaintext media (no SFU) |
| STUN | Untrusted helper | Reflexive addresses | Session tokens |
| Network / ISP / GFW | Adversarial path | Metadata, blocking, correlation | Valid use of expired pairing |

## 3. Policy Enforcement Point

**Invariant.** Android is the final Policy Enforcement Point.

The PC may request `{ screen.read, input.control, audio.read }`.
The coordinator may store the requested and granted sets for lifecycle and UI.
The owner grants a subset.
The device probe further intersects what is actually available.

Only the Android process may execute capture or input:

- If `input.control` is not in the effective set, tap/swipe/back/home/text are dropped locally.
- If AccessibilityService is off, `input.control` is unavailable even if the owner wanted it.
- If capture is protected/black, control is refused by default.
- Mid-session revocation on the phone takes effect immediately. The PC is notified; it is not asked for permission.

**Security requirement.** Treat every DataChannel message as attacker-controlled, including messages that claim a capability the PC thinks is granted.

## 4. Media confidentiality vs signaling authenticity

WebRTC DTLS-SRTP encrypts media between the Android app and the browser.

**Security requirement.** That confidentiality holds against TURN and the network **only if** the DTLS fingerprints in SDP are the real peers’ fingerprints.

| Attacker | Sees frames? | Condition |
|---|---|---|
| Passive network | No | DTLS established |
| TURN (relay, not SFU) | No | DTLS fingerprints authentic |
| Coordinator that can rewrite SDP | **Yes, via MITM** | Unless pairing binds peer identity / fingerprints (SAS) |
| SFU (forbidden in MVP) | Yes | Terminates DTLS by design |

**MVP decision.** No SFU. Coordinator never sits on the media plane.

TLS to the coordinator protects signaling from the network. It does **not** protect media from a compromised coordinator unless pairing binds WebRTC peer identity. See [pairing protocol](../protocol/pairing-protocol.md) and [threat model](threat-model.md).

## 5. Local agent boundary

**MVP decision.** The local Go agent is a distinct component from the public coordinator.

It is a **local security boundary** for operator bootstrap (UI origin, loopback bind, bridging signaling). It is not:

- a second PEP
- a media terminator
- a way for the phone to avoid the public coordinator

**Security requirement (M5 hardening, design now).** Bind `127.0.0.1`, authenticate the local UI, check `Origin`, never expose the agent on LAN interfaces as the product path.

Localhost CSRF / DNS rebinding is a real threat against a dashboard at `http://localhost:3000`. Hardening is scheduled in M5; the topology must not require opening the agent to `0.0.0.0`.

## 6. Coordinator origin trust

**Security requirement.** Android must only contact allowlisted coordinator origins. A QR that embeds an arbitrary HTTPS URL is otherwise a phishing trampoline.

The allowlist is a product security control, not a convenience setting for MVP.

## 7. What “the PC is not authoritative” means in practice

| Event | Wrong behavior | Required behavior |
|---|---|---|
| PC sends tap while view-only | Android executes it | Drop + error |
| Operator UI shows “control allowed” | Android trusts the UI | Android re-checks grant + a11y |
| Coordinator stores `input.control=true` | Android skips local check | Coordinator is informational |
| Owner disables a11y | PC continues sending | Android refuses; `capability_update` |
| Session `CLOSED` | Late packet still injects | PEP dead; no execution |

## 8. Data that may cross each boundary

| Path | Allowed | Forbidden |
|---|---|---|
| Android → browser (WebRTC) | Video (if granted); later optional non-call audio; control acks | Unencrypted frames; a11y tree dumps |
| Browser → Android (DataChannel) | Versioned control envelopes | Capability grants, pairing secrets |
| Either → coordinator | SDP/ICE, session events, cap names, hangup | Frames, audio, command bodies, raw QR secrets |
| Coordinator → TURN auth | Short-lived, session-scoped credentials | Long-term shared TURN password in clients |
| Agent → browser | UI assets, local bootstrap | Media proxy in MVP |
