# PhoneBeam

Secure remote Android phone support from a PC browser.

PhoneBeam is a consent-first, attended remote-support product. A phone owner
explicitly approves a short-lived session; an operator can then view the
Android screen from a PC and, only where Android officially allows, interact
with it. Either side can disconnect immediately.

This is not surveillance software, not unattended or hidden access, and not
an Android security bypass. Capabilities the platform does not expose are
unsupported.

## Current status

Architecture is being reviewed and locked before implementation. This
repository does not yet contain application code.

## Planned stack

- Android companion: Kotlin
- Operator UI: React and TypeScript
- Coordinator: Go
- Realtime media and control: WebRTC
