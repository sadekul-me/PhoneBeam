# M2 — Authenticated realtime viewing

Implementation notes for authenticated WebRTC screen viewing. Locked product
decisions remain in `docs/architecture/`. This file records what M2 actually
ships, what is implemented in code, and what still requires deployed
infrastructure or a physical device.

M1 pairing is unchanged as a prerequisite. M2 does **not** add remote control,
AccessibilityService, audio, recording, accounts, unattended access, persistent
device pairing, an SFU, or screen-over-WebSocket.

**Status:** CODE COMPLETE for the architecture below.
**Runtime infrastructure validation:** OUTSTANDING for physical devices and a
real TURN deployment.

## Architecture

```
Android Phone
      |
      |  WebRTC media (VP8 preferred, video only)
      |
+-----+----------------------+
|                            |
direct ICE              TURN relay
|                            |
+------------+---------------+
             |
             v
      Browser Peer (RTCPeerConnection)
```

Signaling only (no frames):

```
Android  -- authenticated WSS -->  Coordinator  <-- authenticated WSS -- Browser
```

The public Go coordinator:

- authenticates operator vs phone roles
- forwards versioned signaling messages
- distributes ICE configuration and short-lived TURN REST credentials
- owns the session state machine

The coordinator must not receive video frames, decrypt media, proxy screen
buffers, transcode, or become an SFU. Media is Android `PeerConnection` to
browser `RTCPeerConnection`. There is still no local Go agent and no Pion
browser peer.

## Signaling

Path: `GET /api/v1/sessions/{id}/signal` (WebSocket).

Authentication is the first JSON message, not a URL query:

```json
{"v":1,"type":"hello","token":"<operator_token|phone_token>"}
```

Hello is required within 3 seconds. Tokens are the M1 role-separated session
credentials. A token from another session is rejected. Phone-pending tokens
cannot attach.

Message types (`v` must be `1`, max 64 KiB):

| Type | Direction | Notes |
|---|---|---|
| `hello` / `hello_ok` | client → server → client | Role bind |
| `sdp_offer` | phone → operator | Phone is the offerer |
| `sdp_answer` | operator → phone | Browser is the answerer |
| `ice_candidate` / `ice_complete` | both | Trickle ICE |
| `peer_fingerprint` | both → coordinator | Stored; **not** used as the SAS source |
| `peer_ready` | both → coordinator | ICE/DTLS locally ready (`connected`/`completed` only) |
| `projection` | phone | `pending` / `active` / `denied` |
| `stats` | both | Path/bitrate/size only |
| `need_offer` | operator → phone | One re-handshake |
| `hangup` / `failed_ice` / `failed_signaling` | both | Close or fail |
| `state` / `error` | server → client | |

ICE servers are **never** taken from peer signaling. Clients fetch:

`GET /api/v1/sessions/{id}/ice` with `Authorization: Bearer`.

Projection may also be reported over HTTP:

`POST /api/v1/sessions/{id}/projection`

WebSocket limits: 64 KiB messages, ~10 msg/s with burst 30, one socket per
role per session (a new same-role socket replaces the previous). Operator
disconnect starts a **5 second** grace; if the operator does not reattach,
the session hangs up. Phone disconnect hangs up immediately.

## WebRTC offer / answer

- Android phone = offerer
- Browser = answerer
- Trickle ICE
- Video transceiver only (`OfferToReceiveAudio=false`)
- Codec preference: **VP8**, with library/browser fallback if required

## Video source / MediaProjection pipeline

M0 still owns local hub capture (`ImageReader` + one `VirtualDisplay`).

Remote viewing does **not** create a second MediaProjection owner. After M1
owner grant of `screen.read` and the official system capture dialog, 
`CaptureService` starts in remote mode and `ScreenCapturerAndroid` consumes
the same projection result Intent. The M0 ImageReader path is skipped while
`PhoneBeamApp.viewingAuth` is set.

Sequence:

1. Operator creates session, phone scans QR, owner grants `screen.read` → `CAPS_BOUND`
2. Android shows **Start screen sharing** (PhoneBeam consent is not a capture substitute)
3. Official MediaProjection dialog
4. On approve: visible `mediaProjection` FGS + `ScreenCapturerAndroid` + `PROJECTION_ACTIVE`
5. Authenticated signaling, phone offer, browser answer
6. DTLS fingerprints bound into media SAS
7. Browser attaches the remote video track

If the system dialog is denied: `PROJECTION_DENIED`. No WebRTC screen track.
The session stays in that named state so the operator UI can show that screen
sharing was not started. The owner may retry from the consent screen
(`PROJECTION_DENIED` → `PROJECTION_PENDING`).

If projection is revoked after media is live: WebRTC tears down, FGS stops,
coordinator `CLOSED`. No capture token reuse.

## Session lifecycle

Implemented states beyond M1:

`CAPS_BOUND` → `PROJECTION_PENDING` → `PROJECTION_ACTIVE` → `NEGOTIATING` → `CONNECTED`

Named failures: `PROJECTION_DENIED`, `VIEW_NOT_REQUESTED`, `FAILED_ICE`,
`FAILED_SIGNALING`, `PEER_AUTH_FAILED`, then `DISCONNECTING` → `CLOSED`.

`CONNECTED` requires all of:

- pairing approved
- effective caps contain `screen.read`
- projection live
- offer and answer seen
- both DTLS fingerprints recorded
- both peers `peer_ready`

SDP exchange alone does not mark `CONNECTED`. Clients send `peer_ready` only after
`iceConnectionState` is `connected` or `completed`, so the coordinator cannot
advance on offer/answer alone.

## ICE / STUN / TURN

| Piece | Status |
|---|---|
| STUN URI list from coordinator env | IMPLEMENTED |
| Trickle ICE | IMPLEMENTED |
| `iceTransportPolicy=all` default | IMPLEMENTED |
| Force relay (`PHONEBEAM_ICE_TRANSPORT_POLICY=relay`) | IMPLEMENTED (config) |
| coturn REST short-lived credentials | IMPLEMENTED (code) |
| TURN UDP / TCP / TLS URIs | IMPLEMENTED as config (`PHONEBEAM_TURN_URIS`) |
| Deployed coturn / managed TURN | REQUIRES DEPLOYED TURN INFRASTRUCTURE |
| Same-LAN / cellular / China↔Bangladesh runtime | REQUIRES DEPLOYED TURN + devices |

Without `PHONEBEAM_TURN_SECRET` **and** `PHONEBEAM_TURN_URIS`, the coordinator
emits STUN/host only and `turn_configured=false`. It does not fake relay
success.

TURN username is `expiryUnix:sid`. Password is HMAC-SHA1(secret, username)
Base64, matching coturn `--use-auth-secret`. TTL 10 minutes. Secrets are
never placed in the APK, React bundle, QR, or git.

Development template: `templates/coturn.env.example`.

Force-TURN test plan (when a TURN host exists):

1. Set `PHONEBEAM_ICE_TRANSPORT_POLICY=relay` on the coordinator
2. Complete pairing and viewing
3. Confirm diagnostics show `relay`, not `direct`

## DTLS fingerprint / SAS binding

Two distinct codes:

| Code | Domain | When | What it binds |
|---|---|---|---|
| Pairing code | `phonebeam-sas-v1` | After scan / approval | origin, sid, pid, operator, sorted requested caps |
| Verified media code | `phonebeam-sas-v2` | After local+remote DTLS fps | pairing transcript + `android_fp` + `browser_fp` |

Canonical fingerprint: SHA-256 hex, lowercase, colons/spaces stripped, length 64.

Media SAS is computed **locally** on each peer from fingerprints observed on
that peer’s own `PeerConnection`. Coordinator-supplied fingerprint fields are
not the trust source. `sas_material` is the HMAC key (base64url) issued after
`CAPS_BOUND` to operator and phone tokens only.

Golden vector (all three languages): key
`phonebeam-sas-test-key-32bytes!!`, origin `http://127.0.0.1:8080`,
sid `sid-a`, pid `pid-b`, operator `Support-A`, caps
`input.control,screen.read` (sorted), android_fp `0123…cdef`, browser_fp
`fedc…3210` → **`830 034`**.

A coordinator that swaps SDP/ICE toward a media MITM cannot make both humans
see the same media code unless both actually DTLS-handshake with each other
(or accidentally share the same role-labeled fingerprint pair). This is
coordinator-resistant **media authentication**, not a full encrypted identity
system and not a substitute for TLS on the coordinator itself.

Media is treated as trusted in the coordinator (`media_trusted`) only at
`CONNECTED`. Operators must still compare the verified media code.

## Reconnect / refresh

Locked for M2:

- One authenticated WebRTC re-handshake (`need_offer`) on the same session if
  MediaProjection is still live, the same operator token is valid, session TTL
  has not expired, and no other operator identity exists.
- A second `need_offer` closes the session.
- Operator WebSocket drop: 5s grace to reattach; otherwise hangup.
- Full browser document reload drops in-memory operator credentials. After the
  5s grace the session closes. Security over convenience.
- ICE failure while projection remains valid may enter `RECONNECTING` /
  `FAILED_ICE` with that single re-handshake. Android keeps MediaProjection
  live for **20 seconds** waiting for `need_offer`; a second ICE failure after
  the restart hangs up. No infinite retry.
- If projection dies, the session ends. Capture is never silently recreated.

## Disconnect

| Trigger | Result |
|---|---|
| Phone Disconnect / FGS stop | hangup → stop capturer/PC → stop FGS → `CLOSED` |
| Operator Disconnect | hangup → phone closes PC and projection → FGS stop → `CLOSED` |
| System stop-sharing | projection callback → WebRTC teardown → `CLOSED` |
| Session TTL | coordinator `CLOSED` → clients tear down |

No capture survives `CLOSED`.

## Browser UI

After connect: remote `<video>` with `object-fit: contain` (letterboxing, no
stretch), session state, connection path (`connecting` / `direct` / `relay` /
`reconnecting` / `disconnected`), capture scope if known, capabilities,
pairing code vs verified media code, Disconnect.

No tap/swipe/keyboard/control widgets.

Operator token stays in React memory. Not written to `localStorage`.

Development is HTTP/`ws:`. Production design remains HTTPS/WSS.

## Android UX

`ViewingActivity` plus the mandatory ongoing MediaProjection notification
(“PhoneBeam remote support is active”). Stop/Disconnect is available in the
activity and the notification. Hub M0 “Start Screen Share” remains a local
proof and is not started by pairing.

## Capture scope

Android 14+ may share a single app instead of the whole display. PhoneBeam
reports `capture_scope=unknown` unless a reliable platform API is present.
Do not claim “entire phone” without that metadata.

## Configuration

Coordinator:

- `PHONEBEAM_LISTEN`, `PHONEBEAM_ORIGIN`, `PHONEBEAM_CORS_ORIGINS`
- `PHONEBEAM_QR_TTL` (default 120s), `PHONEBEAM_SESSION_TTL` (default 60m)
- `PHONEBEAM_STUN_URIS` (default Google STUN for **dev only**)
- `PHONEBEAM_TURN_URIS`, `PHONEBEAM_TURN_SECRET`
- `PHONEBEAM_ICE_TRANSPORT_POLICY` (`all` or `relay`)

Web: `VITE_COORDINATOR_ORIGIN` (default `http://127.0.0.1:8080`).

Android trusted origins remain the M1 allowlist. LAN testing still requires
adding the origin to `TrustedCoordinators` and `network_security_config.xml`.

## Development test modes

| Mode | How | Auth still required |
|---|---|---|
| Same machine | coordinator on loopback, emulator via `10.0.2.2`, web on `:5173` | yes |
| Same LAN | physical phone + PC, origin allowlisted | yes |
| Different network | cellular / other Wi-Fi + TURN | yes |
| Force TURN | `PHONEBEAM_ICE_TRANSPORT_POLICY=relay` | yes |

Debug modes do not create unauthenticated screen access.

## Test matrix

| Area | What |
|---|---|
| Coordinator unit | signaling auth, wrong role/session, offer/answer/ICE forward, hangup, expiry, CONNECTED gating, rehandshake once, rate limit |
| Protocol | version, SDP/ICE envelopes, fingerprint canonicalization, golden SAS |
| Web | SAS golden vector, aspect `contain`, no remote-control labels, credential-clear states, signaling URL has no token |
| Android unit | SAS golden vector, viewing policy, signaling candidate checks, M0 capture machine |
| Android instrumented / device | NOT VALIDATED in this environment |
| NAT traversal | unit tests do **not** prove it |

## Real-device tests outstanding

The following are **NOT VALIDATED** here (no physical device attached to Cursor):

- MediaProjection system dialog
- Real remote frames in the browser
- Background sharing / other-app capture
- Portrait/landscape rotation of the remote video
- System stop-sharing
- Screen lock while sharing
- Wi-Fi ↔ cellular ICE restart
- Force-TURN on a real relay

M0 local frame proof on a real device was already outstanding before M2.

## Security boundaries

- Coordinator never sees frames
- No SFU
- No remote control / AccessibilityService
- No audio / recording
- No unattended access / boot persistence
- No permanent TURN credentials in repo, APK, bundle, or QR
- Android activities for scan/consent/viewing are `exported=false`
- ICE config is coordinator-issued only
- Production logs must not include SDP, candidates, tokens, TURN passwords, or frames
