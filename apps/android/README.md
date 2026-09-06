# PhoneBeam Android (M0)

Local MediaProjection foundation only. Open this directory in Android Studio or build with the Gradle wrapper.

```
# From apps/android, using Android Studio's JDK 21 if system Java is 25+:
#   set JAVA_HOME to the Studio JBR
gradlew.bat assembleDebug testDebugUnitTest
```

minSdk 29, compile/target SDK 35, namespace `com.phonebeam.android`.

This module must not gain INTERNET, AccessibilityService, WebRTC, or pairing code in M0.
