# PhoneBeam coordinator

Local development:

```
go run ./cmd/coordinator
```

Listens on `127.0.0.1:8080` by default. In-memory only. Restart drops all sessions.

For Android emulator pairing, listen on all interfaces and advertise the emulator alias as the QR origin:

```
set PHONEBEAM_LISTEN=0.0.0.0:8080
set PHONEBEAM_ORIGIN=http://10.0.2.2:8080
go run ./cmd/coordinator
```

Environment:

- `PHONEBEAM_LISTEN` (default `127.0.0.1:8080`)
- `PHONEBEAM_ORIGIN` (QR origin, default `http://127.0.0.1:8080`)
- `PHONEBEAM_CORS_ORIGINS` (comma-separated)
- `PHONEBEAM_QR_TTL` (default `120s`)
- `PHONEBEAM_SESSION_TTL` (default `60m`)
- `PHONEBEAM_STUN_URIS` (default `stun:stun.l.google.com:19302` for development)
- `PHONEBEAM_TURN_URIS` (comma-separated; empty unless a TURN host exists)
- `PHONEBEAM_TURN_SECRET` (coturn REST secret; never commit a live value)
- `PHONEBEAM_ICE_TRANSPORT_POLICY` (`all` or `relay`)

Signaling: `GET /api/v1/sessions/{id}/signal` (WebSocket, token in `hello`).
ICE config: `GET /api/v1/sessions/{id}/ice`.

The coordinator never receives media frames. See
`docs/implementation/m2-authenticated-webrtc-viewing.md`.
