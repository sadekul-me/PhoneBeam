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

Architecture decisions are locked and documented. This repository does not
yet contain application code. Implementation will follow the milestone order
in the architecture docs, starting with M0 (on-device MediaProjection
foundation) only after review.

Do not treat any debug capture path as a product remote stream.

## Architecture

- [System architecture](docs/architecture/system-architecture.md)
- [Android capability matrix](docs/architecture/android-capability-matrix.md)
- [Trust boundaries](docs/architecture/trust-boundaries.md)
- [Session lifecycle](docs/architecture/session-lifecycle.md)
- [Threat model](docs/architecture/threat-model.md)
- [Architecture decisions](docs/architecture/architecture-decisions.md)
- [Pairing protocol](docs/protocol/pairing-protocol.md)
- [Control protocol](docs/protocol/control-protocol.md)

## Planned stack

- Android companion: Kotlin (min API 29)
- Operator UI: React and TypeScript
- Local operator agent: Go (not the WebRTC media peer)
- Public coordinator: Go (session, pairing, signaling — not media)
- Realtime media: WebRTC (Android ↔ browser; TURN required fallback)
