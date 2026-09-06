# PhoneBeam Android (M1)

M0 local MediaProjection remains. M1 adds QR pairing and consent. Pairing does not start capture or send frames.

```
# From apps/android, using Android Studio's JDK 21 if system Java is 25+:
gradlew.bat assembleDebug testDebugUnitTest
```

minSdk 29, compile/target SDK 35, namespace `com.phonebeam.android`.

Development coordinator origins allowed by the QR parser: `http://127.0.0.1:8080`, `http://localhost:8080`, `http://10.0.2.2:8080`. For a physical device, add the LAN origin to `TrustedCoordinators` and `network_security_config.xml`, and set `PHONEBEAM_ORIGIN` on the coordinator to that origin.
