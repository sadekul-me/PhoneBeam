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

**M0 — Android MediaProjection foundation** is in progress.

The architecture baseline is locked. Application code exists only under `apps/android/` and is limited to on-device screen capture: system MediaProjection consent, a visible foreground service, and local frame proof. There is no pairing, coordinator, WebRTC, or remote control yet.

See [M0 implementation notes](docs/implementation/m0-mediaprojection.md).

## Architecture

- [System architecture](docs/architecture/system-architecture.md)
- [Android capability matrix](docs/architecture/android-capability-matrix.md)
- [Trust boundaries](docs/architecture/trust-boundaries.md)
- [Session lifecycle](docs/architecture/session-lifecycle.md)
- [Threat model](docs/architecture/threat-model.md)
- [Architecture decisions](docs/architecture/architecture-decisions.md)
- [M0 implementation notes](docs/implementation/m0-mediaprojection.md)
- [Pairing protocol](docs/protocol/pairing-protocol.md)
- [Control protocol](docs/protocol/control-protocol.md)

## Planned stack

- Android companion: Kotlin (min API 29)
- Operator UI: React and TypeScript
- Local operator agent: Go (not the WebRTC media peer)
- Public coordinator: Go (session, pairing, signaling — not media)
- Realtime media: WebRTC (Android ↔ browser; TURN required fallback)
