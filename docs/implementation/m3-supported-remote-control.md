# M3 — Supported remote control

Implementation notes for optional, consent-first remote control. Locked product
decisions remain in `docs/architecture/` and `docs/protocol/control-protocol.md`.

M2 authenticated viewing is unchanged. M3 does **not** add audio, file transfer,
clipboard sync, keyboard/HID, window-content scraping, unattended access, or
boot persistence.

**Status:** CODE COMPLETE for the architecture below.
**Runtime / physical-device validation:** OUTSTANDING.
**TURN validation:** still OUTSTANDING from M2.

## Architecture

Control uses a reliable ordered WebRTC DataChannel named `phonebeam-control`.

The phone (offerer) creates the channel before SDP offer. The coordinator never
sees command payloads.

```
Browser  -- DataChannel (DTLS/SCTP) -->  Android PEP
                                      --> AccessibilityService (optional)
```

## Prerequisites

A control command executes only when all of these are true:

1. Session is `CONNECTED`
2. Owner granted `input.control` (in the frozen effective set)
3. PhoneBeam AccessibilityService is enabled by the user in Android Settings
4. MediaProjection is still live
5. Keyguard is not locked
6. Capture is not heuristically protected/black
7. Envelope validates (version, sid, seq, body, rate limit)

View-only sessions remain first-class. Missing Accessibility is not a failed
viewing session.

## AccessibilityService

`PhoneBeamAccessibilityService`:

- `canRetrieveWindowContent=false`
- `isAccessibilityTool=false`
- `canPerformGestures=true`
- exported only with `BIND_ACCESSIBILITY_SERVICE`
- no window-content / node-tree scraping

Supported actions:

- `dispatchGesture` tap
- `dispatchGesture` swipe
- `GLOBAL_ACTION_BACK`
- `GLOBAL_ACTION_HOME`

The user must enable the service in system Accessibility settings. PhoneBeam
does not navigate Settings or auto-enable it.

`device_available` at pairing includes `input.control` because the APIs exist.
Live availability is narrower and advertised on the DataChannel as
`capability_update`.

## Protocol

Versioned JSON envelope (`v=1`):

`v`, `sid`, `seq`, `ts`, `cap`, `type`, `body`

Types: `tap`, `swipe`, `back`, `home`, `ping`, `pong`, `capability_update`,
`session_close`, `ack`, `error`.

Golden tap vector (Kotlin + TypeScript):

```json
{"v":1,"sid":"sid-a","seq":7,"ts":1700000000000,"cap":"input.control","type":"tap","body":{"x":0.25,"y":0.75}}
```

Coordinates are normalized `[0,1]` in **encoded video frame** space. The operator
UI maps pointer events through `object-fit: contain` and rejects letterbox hits.

Android maps `[0,1]` onto the capture frame size used for `ScreenCapturerAndroid`.
Ambiguous geometry returns `unsupported_geometry`.

## PEP

Every inbound DataChannel message is evaluated in `CommandPep` before any
gesture. Claims in `cap` are not grants. Stable errors:

`unsupported_version`, `unknown_type`, `invalid_body`, `replay`,
`session_inactive`, `capability_denied`, `unsupported_on_device`,
`unsupported_geometry`, `protected_content`, `rate_limited`

Limits: 8 commands/s, one gesture in flight, swipe duration 50–2000 ms, strict
monotonic `seq`.

## Protected content

Default refuse inject when:

- keyguard locked
- local capture frames are uniformly near-black for several consecutive frames
  (FLAG_SECURE heuristic from the WebRTC local video track)
- no sampled frame yet

This does **not** use the accessibility tree. The heuristic can miss
non-black protected UI and can false-positive a genuinely black wallpaper.
Those cases fail closed or may be slightly over-refused; they are documented
limitations, not bypasses.

## Revocation

If Accessibility is disabled mid-session:

- live `input.control` is removed
- `capability_update` is sent
- further control commands are rejected
- screen viewing continues if `screen.read` remains valid

## Operator UI

- VIEW ONLY unless live caps include `input.control` and state is `CONNECTED`
- click → tap, click-drag → swipe
- Back / Home buttons only when live
- no keyboard
- ack/error codes are shown; success is never faked

## Disconnect

`session_close` on the DataChannel hangs up the same path as signaling hangup:
WebRTC, projection, FGS, coordinator `CLOSED`.
