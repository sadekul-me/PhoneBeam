# M1 — Secure session foundation

Implementation notes for pairing, consent, capability binding, and the public coordinator. Locked product decisions remain in `docs/architecture/`. This file records what M1 actually ships.

M0 local MediaProjection is unchanged as a separate on-device proof. Pairing does **not** start capture, WebRTC, TURN, AccessibilityService, or remote input.

There is **no local Go agent** in M1. The development operator UI talks to the coordinator over HTTP.

## Coordinator

`services/coordinator` is an in-memory Go service.

Default listen: `127.0.0.1:8080`  
QR origin: `PHONEBEAM_ORIGIN` (default `http://127.0.0.1:8080`)  
CORS: `PHONEBEAM_CORS_ORIGINS` (default `http://localhost:5173,http://127.0.0.1:5173`)

Restart drops every session. That is acceptable for M1.

Emulator testing:

- Listen on `0.0.0.0:8080`
- Set `PHONEBEAM_ORIGIN=http://10.0.2.2:8080` so the QR origin is what the phone will contact
- Operator UI still uses `http://127.0.0.1:8080`

Physical-device LAN testing requires adding that origin to Android `TrustedCoordinators` and `network_security_config.xml`.

## API (`/api/v1`)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | none | liveness |
| POST | `/api/v1/sessions` | none | create session + operator token + QR |
| GET | `/api/v1/sessions/{id}` | Bearer operator, phone, or pending | public state |
| POST | `/api/v1/sessions/{id}/scan` | body `{ "pid" }` | first valid scan |
| POST | `/api/v1/sessions/{id}/approve` | Bearer phone pending | grant subset, bind caps |
| POST | `/api/v1/sessions/{id}/reject` | Bearer phone pending | owner reject |
| POST | `/api/v1/sessions/{id}/close` | Bearer operator or phone | hangup |

JSON only. 16 KiB body limit. Unknown fields rejected. Live updates are **1s polling** from the operator UI.

Role tokens are separate:

- `operator_token` — create/get/close
- `phone_pending_token` — approve/reject only while `APPROVAL_PENDING`
- `phone_token` — issued at approve; pending token dies

The coordinator never logs tokens, pairing secrets, or full session dumps.

## Session state machine

Pure table in `internal/session/machine.go`. HTTP handlers cannot add edges.

Implemented session states:

`SESSION_CREATED` → `QR_AVAILABLE` → `PHONE_SCANNED` → `APPROVAL_PENDING` → `APPROVED` → `CAPS_BOUND` → `CLOSED`

Terminal: `REJECTED`, `EXPIRED`, `CLOSED`

`CONNECTED` is reachable in M2 only after projection, authenticated SDP, and DTLS/peer-ready conditions. See [M2 implementation notes](m2-authenticated-webrtc-viewing.md).

`REJECTED_CONSUMED` is the **losing scanner's** outcome, not a session-wide abort. The first valid scan keeps `APPROVAL_PENDING`. The second concurrent scan receives HTTP `409` / `rejected_consumed`. That matches exclusive lock without kicking the winner.

Invalid transitions return `InvalidTransitionError`.

## QR payload and TTLs

QR JSON (protocol v1):

```json
{"v":1,"origin":"http://127.0.0.1:8080","sid":"...","pid":"...","exp":1700000120}
```

Not in the QR: passwords, long-term keys, TURN secrets, session tokens, reusable room codes.

The one-time pairing lock is server-side (`pid` + scan lock + burn). Photographing a consumed QR cannot approve.

**Pairing / QR TTL: 120 seconds.** Short enough that a screenshot ages out quickly; long enough to scan and read SAS. Applies from QR issue through `APPROVAL_PENDING`. On fire → `EXPIRED`.

**Session TTL: 60 minutes, starting at owner approval / `CAPS_BOUND`.** Pairing time does not consume the support-session budget. QR create does not start the 60-minute clock. After approval the session moves to `CLOSED` when that TTL fires (M1 has no `DISCONNECTING` media teardown).

## Scan lock / replay

1. Create issues `sid` + `pid`, state `QR_AVAILABLE`.
2. First valid `{sid,pid}` scan under the mutex takes the lock, issues `phone_pending_token`, state `APPROVAL_PENDING`.
3. Concurrent second scan fails `ErrScanLocked` (`rejected_consumed`). Winner is unchanged.
4. Approve burns `pid`, drops pending token, issues `phone_token`.
5. Reject also burns `pid`.
6. Timeout burns `pid` and moves to `EXPIRED`.
7. Replay of a burned/expired `pid` fails closed. Closed sessions cannot reopen.

## SAS

M1 SAS is a 6-digit grouped code `NNN NNN`.

Construction: HMAC-SHA256 over UTF-8 transcript, keyed by a coordinator-held random 32-byte key:

```
phonebeam-sas-v1
origin=...
sid=...
pid=...
operator=...
caps=comma-sorted requested caps
```

Display: first 4 digest bytes interpreted as big-endian uint32, reduced modulo 1_000_000.

Both UIs show the coordinator-computed value. This binds **pairing/session identity**, not WebRTC.

**M1 limitation:** a compromised coordinator can still show matching SAS on both screens. That is why M2 must extend the transcript with WebRTC DTLS fingerprints before media is trusted. Do not treat M1 SAS as a DTLS peer bind.

## Capability grant model

Known names: `screen.read`, `input.control`, `audio.read`.

```
effective = requested ∩ owner_granted ∩ device_available
```

The coordinator cannot add names. Granting an unrequested name is rejected. Android reports M1 device availability as `screen.read` only (`input.control` and `audio.read` are not implemented). Intersection, not union.

M1 does not execute any capability.

## Android M1 flow

Hub still has M0 **Start Screen Share**.

New: **Scan Pairing QR** → CameraX + ML Kit QR → `PairingQrParser` (PhoneBeam JSON only; no URL / WebView / external intent) → consent screen (operator identity, requested caps, SAS) → grant subset or reject → show resulting coordinator state.

Trusted development origins: `http://127.0.0.1:8080`, `http://localhost:8080`, `http://10.0.2.2:8080`.

`INTERNET` is used only for coordinator JSON. Pairing never calls `CaptureService`.

## Web M1 flow

`apps/web` (React + TypeScript):

Create session → choose requested caps → show QR + 120s countdown → poll state → after scan show SAS → show approved/rejected/expired → close session.

No fake phone screen. No WebRTC.

## Known limitations / what M2 must add

- No WebRTC, STUN, TURN, or media path
- No DTLS fingerprint binding in SAS
- In-memory store only
- Development HTTP origins / cleartext allowlist
- No production TLS, no persistent sessions
- No local operator agent
- Polling rather than push
- Android allowlist is a compile-time development set
- Capabilities are recorded, not enforced on a media/control channel
