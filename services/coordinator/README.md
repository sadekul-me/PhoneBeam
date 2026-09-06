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
