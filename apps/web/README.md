# PhoneBeam operator UI (M3)

Development:

```
npm install
npm run dev
```

Set `VITE_COORDINATOR_ORIGIN` if the coordinator is not `http://127.0.0.1:8080`.

This UI creates pairing sessions, shows QR / pairing SAS, displays the remote
Android screen over authenticated WebRTC, and — only when `input.control` is
live — maps click/drag plus Back/Home onto a DataChannel. There is no keyboard.

Operator tokens stay in memory. Development uses HTTP/`ws:`; production design
is HTTPS/WSS.
