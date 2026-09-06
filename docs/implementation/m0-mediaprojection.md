# M0 — Android MediaProjection foundation

Implementation notes for the first coding milestone. Locked product decisions remain in `docs/architecture/`. This file records what the Android module actually does.

## Toolchain

| Item | Choice | Reason |
|---|---|---|
| Namespace / applicationId | `com.phonebeam.android` | Matches the locked PhoneBeam identity |
| minSdk | 29 (Android 10) | Locked floor |
| compileSdk / targetSdk | 35 | Local SDK has `android-35` and `android-36.1`. `android-36` is not installed, so 35 is the unambiguous stable platform |
| AGP | 8.13.2 | Supports API 35 and 36.1 |
| Gradle | 8.13 | Required by AGP 8.13 |
| Kotlin | 2.1.20 | Compatible with AGP 8.13 |
| JDK for Gradle | 17 bytecode / Android Studio JBR 21 to run Gradle | System `java` is 25, which AGP 8.13 does not treat as the default |

Debug builds use applicationId `com.phonebeam.android.debug` so a later store build can coexist.

## MediaProjection approach

1. Owner taps **Start Screen Share**.
2. On API 33+, `POST_NOTIFICATIONS` is requested if missing. Capture is refused if it is denied (the notification is the visible sharing indicator).
3. The official `MediaProjectionManager.createScreenCaptureIntent()` dialog is launched. There is no fake in-app consent UI.
4. On denial, state becomes `STOPPED`. Nothing is captured.
5. On approval, `CaptureService` is started as a `mediaProjection` foreground service (`START_NOT_STICKY`).
6. The service calls `startForeground` with `FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION`, then `getMediaProjection` **once**, registers `MediaProjection.Callback` **before** `createVirtualDisplay`, and creates one `VirtualDisplay` into an `ImageReader`.
7. The result Intent is not stored for a later session. A new start always re-prompts Android.

Android 14+ one-shot rule: rotation does **not** call `createVirtualDisplay` again. The service resizes the existing `VirtualDisplay` and swaps the `ImageReader` surface.

## Foreground service

- Manifest: `foregroundServiceType="mediaProjection"`, `exported="false"`.
- Ongoing notification: “PhoneBeam is sharing this screen” with a Stop action.
- Notification and service are removed when capture ends.
- No boot receiver, no `START_STICKY`, no silent restart after process death.

## Local capture proof (no network)

A full-screen live preview of the same display would recurse (infinity mirror) and would look like a stream endpoint.

M0 proof is therefore:

- running frame count, size, DPI, and fps in the app
- a **downscaled last frame** kept only in memory (not written to disk)
- capture continues in the FGS while the activity is backgrounded, so switching to another app then returning should show that app in the last-frame thumbnail and a higher frame count

There is no HTTP, WebSocket, WebRTC, or file recording in this module. The app does not declare `INTERNET`.

## Lifecycle

| Event | Behavior |
|---|---|
| User Stop in app or notification | `STOPPING` → tear down → `STOPPED` |
| `MediaProjection.Callback.onStop()` (system UI, lock, other projection) | Fail closed → `STOPPED`; new system consent required |
| Activity background | Capture **continues** (FGS). This is required to prove other-app frames locally |
| Screen lock | Typically `onStop()`; no recapture after unlock |
| Process / service death | `START_NOT_STICKY`; no cached token |

## State machine

`IDLE` → `REQUESTING_PERMISSION` → `STARTING` → `ACTIVE` → `STOPPING` → `STOPPED` (or `ERROR`).

Illegal: `IDLE` → `ACTIVE`, or `STOPPED` → `ACTIVE`, without a new permission grant. Covered by JVM unit tests.

## Tests that JVM cannot honest-prove

- Real MediaProjection dialog and token one-shot behavior
- VirtualDisplay buffers on a physical display
- Screen-lock `onStop()`
- OEM battery killers
- Single-app vs whole-screen capture on API 34+
- Emulators (often block or fake MediaProjection)

Those require a **real device**.

## Known limitations

- Foldables / Dex / external displays are not specially handled; display changes use default-display metrics.
- Live thumbnail recurses while PhoneBeam is the captured app.
- `FLAG_SECURE` apps appear black; that is Android, not a capture bug.
- API 34+ users may share one app instead of the whole screen.
- Aggressive OEMs may still kill the FGS; that is documented in the capability matrix, not bypassed.
