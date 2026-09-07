# PhoneBeam

Secure remote Android phone support from a PC browser.

PhoneBeam is a consent-first, attended remote-support product. A phone owner
explicitly approves a short-lived session; an operator can then view the
Android screen from a PC and, only where Android officially allows, interact
with it. Either side can disconnect immediately.

This is not surveillance software, not unattended or hidden access, and not
an Android security bypass. Capabilities the platform does not expose are
unsupported.

## Current phase

**M2 — Authenticated realtime viewing** is implemented in code.

M0 local MediaProjection remains as an on-device proof. M1 pairing, consent, and capability grants remain required before any remote video. M2 adds authenticated WebRTC viewing from the paired browser. There is still no remote input, AccessibilityService, audio, recording, or unattended access.

See [M2 implementation notes](docs/implementation/m2-authenticated-webrtc-viewing.md), [M1 implementation notes](docs/implementation/m1-secure-session.md), and [M0 implementation notes](docs/implementation/m0-mediaprojection.md).

## Architecture

- [System architecture](docs/architecture/system-architecture.md)
- [Android capability matrix](docs/architecture/android-capability-matrix.md)
- [Trust boundaries](docs/architecture/trust-boundaries.md)
- [Session lifecycle](docs/architecture/session-lifecycle.md)
- [Threat model](docs/architecture/threat-model.md)
- [Architecture decisions](docs/architecture/architecture-decisions.md)
- [M0 implementation notes](docs/implementation/m0-mediaprojection.md)
- [M1 implementation notes](docs/implementation/m1-secure-session.md)
- [M2 implementation notes](docs/implementation/m2-authenticated-webrtc-viewing.md)
- [Pairing protocol](docs/protocol/pairing-protocol.md)
- [Control protocol](docs/protocol/control-protocol.md)

## Planned stack

- Android companion: Kotlin (min API 29)
- Operator UI: React and TypeScript
- Local operator agent: Go (not the WebRTC media peer)
- Public coordinator: Go (session, pairing, signaling — not media)
- Realtime media: WebRTC (Android ↔ browser; TURN required fallback)
