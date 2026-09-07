# PhoneBeam operator UI (M2)

Development:

```
npm install
npm run dev
```

Set `VITE_COORDINATOR_ORIGIN` if the coordinator is not `http://127.0.0.1:8080`.

This UI creates pairing sessions, shows QR / pairing SAS, then displays the
remote Android screen over authenticated WebRTC after owner approval and
system MediaProjection consent. There is no remote control.

Operator tokens stay in memory. Development uses HTTP/`ws:`; production design
is HTTPS/WSS.
