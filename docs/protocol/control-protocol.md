# Control protocol (conceptual)

Status: architecture for M3–M4. Not implemented in this documentation milestone. No DataChannel in M0–M2.

Legend: **Invariant** · **MVP decision** · **Platform limitation** · **Security requirement** · **Future / non-MVP**

## 1. Role of the protocol

After an authenticated WebRTC session exists (M2), the operator browser may send commands on a DataChannel. Android executes **only** what the PEP allows.

**Invariant.** This protocol is untrusted input. Capability names inside a message are claims, not grants.

**MVP decision.** Versioned JSON envelope. Coordinates are normalized `[0,1]` in **encoded video frame** space, not CSS pixels and not raw Android pixels.

## 2. Envelope

Conceptual fields:

```
v       protocol major version (unknown major → fail closed)
sid     session id (must match live session)
seq     monotonic uint64 from sender
ts      client timestamp, advisory only
cap     capability the sender believes is required
type    message type
body    type-specific object
```

**MVP decision.** JSON for MVP. A binary/protobuf encoding is **future / non-MVP** unless this envelope proves too costly.

## 3. Message types by milestone

| Type | Milestone | Required cap | Notes |
|---|---|---|---|
| `tap` | M3 | `input.control` | Point in frame space |
| `swipe` | M3 | `input.control` | Path or start/end + duration |
| `back` | M3 | `input.control` | `performGlobalAction` |
| `home` | M3 | `input.control` | `performGlobalAction` |
| `session_close` | M3 | none (session control) | Either peer |
| `capability_update` | M3/M4 | n/a (phone → PC) | Effective caps changed |
| `ping` / `pong` | M3 | none | Liveness |
| `ack` / `error` | M3 | n/a | Result of discrete commands |
| `long_press` | M3 or M4 | `input.control` | If not in first control slice, keep the type reserved |
| `scroll` | M4 | `input.control` | Not mouse-wheel HID |
| `text_input` | M4 | `input.control` | Unicode into compatible focused field |
| `recents` | future | `input.control` | Optional; do not promise |

No audio control messages in early MVP. No “unlock”, “confirm_system_dialog”, “read_tree”, or “grant_permission” types — ever, for this product class.

## 4. Coordinate system

```
x ∈ [0, 1], y ∈ [0, 1]
origin: top-left of the encoded video frame
```

Android maps through: encoded frame → VirtualDisplay / letterboxing → device pixels, using current rotation.

If mapping is ambiguous (unexpected crop, fold hinge, no current frame size):

```
error.code = unsupported_geometry
```

**Platform limitation.** Hover and right-click have no protocol mapping. Mouse wheel may be translated to `scroll` later; it is not a separate HID axis in v1.

## 5. Android PEP checks (every command)

**Security requirement.** Apply in this order; fail closed.

1. Session is `CONNECTED` (or allow only `ping` / `session_close` during `RECONNECTING`).
2. `sid` matches.
3. `v` supported.
4. `seq` not replayed (strict increase for reliable commands; small window only if an unordered pointer stream is added later).
5. Required capability is in the **effective** set (grant ∩ device availability ∩ not revoked).
6. AccessibilityService available when the command needs it.
7. Capture is not in protected/black state (default refuse inject).
8. Body schema and numeric ranges valid.
9. Rate limit (gestures in flight, taps per second). `dispatchGesture` cancels in-progress gestures — floods are both a UX and abuse issue.
10. Execute or return `error`.

The coordinator is not on this path. The PC UI is not on this path.

## 6. Reliability and acknowledgements

| Class | Delivery | ACK |
|---|---|---|
| Discrete actions (`tap`, `swipe`, `back`, `home`, `text_input`, `scroll`) | Reliable DataChannel | `ack` with `seq` or `error` |
| `session_close` | Reliable | Hang up even if ack is lost |
| `ping`/`pong` | Either | `pong` |
| `capability_update` | Reliable phone → PC | Optional for MVP |

Do not queue a backlog of gestures across `RECONNECTING`. Drop or error; operator retries.

## 7. Errors (stable codes)

| Code | Meaning |
|---|---|
| `unsupported_version` | Unknown `v` |
| `unknown_type` | Unknown `type` |
| `invalid_body` | Schema / range |
| `replay` | `seq` reused or stale |
| `session_inactive` | Not connected |
| `capability_denied` | PEP refused |
| `unsupported_on_device` | Probe failed (no a11y, no focus, etc.) |
| `unsupported_geometry` | Cannot map coordinates |
| `protected_content` | Black/`FLAG_SECURE` refuse-inject |
| `rate_limited` | Too many commands |

The operator UI must surface these. Silent retry loops are not “reliability.”

## 8. Text input (M4)

**MVP decision.** `text_input.body.text` is Unicode. It is insertion into a compatible focused field, not scancode HID emulation.

If there is no focused editable control, return `unsupported_on_device`. Password fields, WebViews, games, and custom IMEs may fail. CJK IME composition is a **platform limitation**; fail honestly.

## 9. Revocation

If the owner disables Accessibility or revokes `input.control`:

- PEP immediately refuses new control messages
- phone sends `capability_update`
- PC must disable control chrome
- viewing may continue if `screen.read` remains effective

Widening caps requires a new pairing, not a control message.

## 10. Out of scope for this protocol

- File transfer
- Clipboard sync as a stealth channel
- Accessibility node trees
- Auto-clicking system dialogs
- Commands that imply root
- Audio in M0–M4
