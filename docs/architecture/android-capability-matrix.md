# Android capability matrix

This matrix is an honest inventory of what PhoneBeam may claim. It is not a feature backlog.

Legend: **Invariant** · **MVP decision** · **Platform limitation** · **Security requirement** · **Future / non-MVP**

## 1. Viewing and control are different problems

**Invariant.** `screen.read` and `input.control` are separate capabilities with separate Android APIs, consents, failure modes, and Play-policy weight.

| Capability | Official mechanism | PhoneBeam consent | Android system consent |
|---|---|---|---|
| `screen.read` | MediaProjection + `mediaProjection` foreground service | Owner grants `screen.read` | System capture dialog **every session** |
| `input.control` | Optional AccessibilityService (`dispatchGesture`, `performGlobalAction`) | Owner grants `input.control` | User enables the service in Android Settings |
| `audio.read` | Playback capture and/or microphone — not call audio | Owner grant (M5+) | Runtime / capture rules as applicable |

**MVP decision.** A session with `screen.read` and without `input.control` is a valid first-class PhoneBeam session.

**Security requirement.** PhoneBeam in-app approval does not replace the MediaProjection system dialog. The product must never cache a projection token to skip that dialog after process death, lock-stop, or reboot.

## 2. Screen capture (MediaProjection)

| Topic | Class | Notes |
|---|---|---|
| Capture API | **Supported with restrictions** | Official `MediaProjection` → `VirtualDisplay` → encoder / WebRTC capturer. |
| Min API | **MVP decision:** API 29 | Android 10 is the floor. Emulators often misrepresent capture. |
| Foreground service | **Platform limitation** | Must use `FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION`. The notification is part of the visible session indicator. |
| Per-session system consent | **Platform limitation** | Required every capture session. Android 14+ treats the result token as one-shot; reuse throws. |
| Whole screen vs one app | **Platform limitation** (API 34+) | The owner may share a single app. Operator UI must show capture scope when known. |
| Rotation | **Supported with restrictions** | Must be handled in M0. Coordinate mapping later uses the **encoded frame**, not a stale orientation. |
| Screen lock | **Platform limitation** | Projection typically stops. Lock screen is not a capture target. Session must not silently recapture after unlock. |
| Projection stop | **Platform limitation** | User notification action, `MediaProjection.Callback.onStop()`, or another projection can end capture. That is real termination of viewing, not a background retry loop. |
| Process death | **Platform limitation** | Projection token is gone. New owner + system consent required. |
| Reboot | **Security requirement** | Session is dead. No `BOOT_COMPLETED` reconnect. |
| OEM battery / background kill | **Device dependent** | Aggressive OEMs will stop the FGS. Setup guidance later; not a bypass. |
| Local preview (M0) | **MVP decision** | On-device / debug-only. Must not become a product remote stream. |

**Platform limitation.** `FLAG_SECURE`, banking, and DRM content appear as black frames in MediaProjection. That is not a PhoneBeam bug. The operator should see a protected-content state.

## 3. Remote control (AccessibilityService)

**MVP decision.** Control is optional and arrives in M3. It uses AccessibilityService only within officially supported capabilities.

**Security requirement.** PhoneBeam is not an accessibility tool. `isAccessibilityTool` must remain false. Play prominent disclosure applies when control ships. Android 17 Advanced Protection may revoke non-tool accessibility access; view-only must still work.

| Topic | Class | Notes |
|---|---|---|
| Tap / swipe | **Supported with restrictions** | `dispatchGesture`. OEM overlays, animation scale, split screen, Dex, foldables reduce reliability. |
| Back / Home / Recents | **Supported with restrictions** | `performGlobalAction`. Usually the most reliable navigation when a11y is enabled. |
| Scroll | **Supported with restrictions** (M4) | Gesture or mapped wheel; not a mouse-wheel HID. |
| Notifications shade | **Device / version dependent** | Opening may work; interacting inside is unreliable. Do not promise it. |
| System / permission dialogs | **Device dependent** | **Security requirement:** never auto-confirm system permission dialogs. |
| Lock screen / keyguard | **Not supported** | Owner unlocks locally. |
| Pointer hover / right-click | **Not supported** | No honest 1:1 touch mapping. |
| Universal keyboard / HID | **Not promised** | **MVP decision:** later text means supported Unicode insertion into compatible focused fields (`ACTION_SET_TEXT` / paste). WebViews, games, custom IMEs, and many password fields will fail visibly. |
| CJK / IME composition | **Platform limitation** | First-class reliability risk for China-based operators. Fail honestly. |
| Window-content / screen text | **Out of MVP** | `canRetrieveWindowContent` can leak text from UIs that MediaProjection blacks out. Forbidden for MVP. |
| Blind inject into black/`FLAG_SECURE` | **Refused by default** | **Security requirement.** If capture is protected/black, do not inject. Seeing nothing and tapping anyway is closer to a RAT than to support. |
| Root / instrumentation / exploit input | **Not supported** | **Invariant.** |

## 4. Audio

| Topic | Class | Notes |
|---|---|---|
| Early MVP | **Out of scope** | No audio track in M0–M4. |
| App playback capture | **Future / restricted** | `AudioPlaybackCapture` (API 29+). Apps can opt out. M5 at earliest. |
| Microphone | **Future / restricted** | Separate grant and legal surface. Not implied by `screen.read`. |
| In-call / VoIP audio | **Not supported** | **Invariant for this product class.** Play policy forbids Accessibility for remote call recording. Do not pursue. |

## 5. Protected content and sensitive UI

| Surface | Viewing | Control default |
|---|---|---|
| `FLAG_SECURE` / banking / DRM | Black frames (**platform limitation**) | **Refuse inject** |
| PhoneBeam consent UI | Normal | Local owner only |
| Android permission dialogs | May be captured | **Do not auto-click** |
| Lock screen | Do not treat as supported capture | Unsupported |

**Security requirement.** Do not scrape the accessibility tree to “compensate” for black frames.

## 6. Lifecycle and background execution

| Event | Required product behavior |
|---|---|
| Owner stops projection | Viewing ends; no silent restart |
| Screen lock | Capture stops; no lock-screen streaming |
| Process death | No silent recapture; new consents |
| Reboot | Session terminal; no boot persistence |
| Network change | M4: ICE restart **only while** the same projection grant is alive |
| Session TTL (60 min default) | Hard stop even if both peers stay connected |
| Accessibility disabled mid-session | `input.control` becomes unavailable immediately (PEP) |

## 7. Device and OEM variance

**Platform limitation.** Do not document a universal control guarantee.

Expect variance in:

- Xiaomi / Oppo / Vivo / Huawei background restrictions
- Samsung and Dex / multi-window geometry
- Huawei devices without GMS (distribution and WebRTC stack)
- Foldables, cutouts, external displays
- Android 14+ single-app capture
- Android 17 Advanced Protection vs AccessibilityService

M5 includes an OEM/device matrix. Until then, capability negotiation must expose `device_available`, not only `owner_granted`.

## 8. What PhoneBeam will not claim

- Unrestricted remote desktop
- Silent or unattended access
- Capture without the system MediaProjection dialog
- Control without Accessibility enabled and `input.control` granted
- Control of every app, including protected and banking surfaces
- Hardware-keyboard fidelity
- Call-audio monitoring
- Persistence across reboot
