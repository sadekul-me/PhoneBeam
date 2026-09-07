# PhoneBeam Android (M2)

M0 local MediaProjection remains as a hub proof. M1 QR pairing and consent remain required. M2 remote viewing starts only after the owner grants `screen.read` **and** approves the official Android MediaProjection dialog.

```
# From apps/android, using Android Studio's JDK 21 if system Java is 25+:
gradlew.bat assembleDebug testDebugUnitTest
```

minSdk 29, compile/target SDK 35, namespace `com.phonebeam.android`.

WebRTC library: `io.github.webrtc-sdk:android` (maintained WebRTC.org Android build). Video only; VP8 preferred.

Development coordinator origins allowed by the QR parser: `http://127.0.0.1:8080`, `http://localhost:8080`, `http://10.0.2.2:8080`. For a physical device, add the LAN origin to `TrustedCoordinators` and `network_security_config.xml`, and set `PHONEBEAM_ORIGIN` on the coordinator to that origin.
