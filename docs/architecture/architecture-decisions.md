# Architecture decisions

Locked baseline. Changing an **Invariant** requires a product decision, not a quiet code change. Changing an **MVP decision** requires updating this file first.

Legend: **Invariant** · **MVP decision** · **Platform limitation** · **Security requirement** · **Future / non-MVP**

## How to read this file

Each entry records: decision, status, recommended/locked choice, alternatives, tradeoff, and whether it blocks coding of the relevant milestone.

---

### D1. Product class

**Decision.** PhoneBeam remains attended, consent-first remote support.

**Status.** **Invariant**

**Choice.** Explicit per-session owner approval. Either side disconnects. No hidden, unattended, or persistent covert access.

**Alternatives.** Unattended agent; MDM device-owner control.

**Tradeoff.** Weaker than AnyDesk-style persistence; aligned with Play policy and owner control.

**Blocks.** All milestones.

---

### D2. Android is the PEP

**Decision.** The PC may request capabilities; Android decides whether commands execute.

**Status.** **Invariant** · **Security requirement**

**Choice.** Effective caps = requested ∩ granted ∩ device_available ∩ not_revoked, enforced on device.

**Alternatives.** Coordinator as PDP+PEP; PC as authority.

**Tradeoff.** A compromised PC or coordinator cannot inject by assertion.

**Blocks.** M1 (grant model), M3 (command gate).

---

### D3. Viewing and control are separate

**Decision.** Remote viewing and remote control are separate capabilities.

**Status.** **Invariant** · **Platform limitation**

**Choice.** `screen.read` via MediaProjection. `input.control` via optional AccessibilityService. View-only sessions are first-class.

**Alternatives.** One “remote desktop” flag.

**Tradeoff.** More UX honesty; matches Android’s actual APIs.

**Blocks.** All capability-related work.

---

### D4. Screen capture API

**Decision.** Official MediaProjection only.

**Status.** **Invariant** · **Platform limitation**

**Choice.** System consent every session; `mediaProjection` FGS; no token reuse after stop/death.

**Alternatives.** Accessibility screenshots; root capture; silent APIs.

**Tradeoff.** Visible, legitimate, lifecycle-heavy.

**Blocks.** M0.

---

### D5. Remote control API

**Decision.** Optional AccessibilityService within officially supported capabilities.

**Status.** **MVP decision** · **Platform limitation** · **Security requirement**

**Choice.** `dispatchGesture` + `performGlobalAction`. Not an accessibility tool (`isAccessibilityTool` stays false). Window-content scraping out of MVP. Refuse control into protected/black screens by default.

**Alternatives.** View-only product; enterprise/MDM APIs; exploit input.

**Tradeoff.** Useful support on many phones; Play disclosure and OEM variance.

**Blocks.** M3. Not M0–M2.

---

### D6. Forbidden techniques

**Decision.** No root, exploit, hidden access, unattended access, boot persistence, silent capture, or Android security bypass.

**Status.** **Invariant**

**Choice.** Unsupported if it requires those.

**Alternatives.** None for this product.

**Blocks.** All milestones (by exclusion).

---

### D7. PC media peer

**Decision.** Browser `RTCPeerConnection` is the PC-side WebRTC media peer for MVP.

**Status.** **MVP decision**

**Choice.** Android ↔ browser DTLS-SRTP. Go local agent is **not** the media peer.

**Alternatives.** Pion in the agent; media server/SFU.

**Tradeoff.** Lower latency, standard WebRTC; recording/transcode later would need a new ADR.

**Blocks.** M2.

---

### D8. Agent vs coordinator

**Decision.** Local Go agent and public Go coordinator are separate components.

**Status.** **MVP decision** · **Security requirement**

**Agent:** serve/launch operator UI; localhost integration; local session/bootstrap; local security boundary; coordination bridge.

**Coordinator:** session creation; pairing; authenticated signaling; lifecycle; short-lived TURN credentials; rate limiting. Never receives screen/audio frames in the normal architecture.

**Alternatives.** Coordinator only on the operator laptop; hosted UI with no agent; agent-as-media-peer.

**Tradeoff.** Extra process on the PC, but the phone can reach a public pairing/signaling plane. A laptop-only coordinator cannot serve Bangladesh without becoming an ad-hoc relay.

**Blocks.** M1 (coordinator), operator UX packaging (agent can lag the public coordinator slightly but not replace it).

---

### D9. Media topology

**Decision.** Android ↔ browser WebRTC. Prefer direct ICE. TURN is the required fallback. No SFU for MVP.

**Status.** **MVP decision** · **Security requirement**

**Choice.** P2P DTLS-SRTP; TURN relays ciphertext; coordinator is signaling-only.

**Alternatives.** SFU; video-over-WebSocket through coordinator.

**Tradeoff.** TURN cost and ops vs E2E confidentiality. SFU would see plaintext.

**Blocks.** M2.

---

### D10. TURN milestone and networks

**Decision.** TURN is part of the first real internet WebRTC milestone (M2), including TCP/TLS 443 for realistic China ↔ Bangladesh paths.

**Status.** **MVP decision** · **Platform limitation** (networks)

**Choice.** STUN + TURN UDP + TURN TCP/TLS 443 in M2. Different-network testing is a gate, not a later surprise.

**Alternatives.** P2P-only M2; TURN in M4/M5.

**Tradeoff.** M2 is heavier, but otherwise the product is validated on the wrong network.

**Blocks.** M2. TURN **provider/region** is still unresolved (see open decisions).

---

### D11. Pairing before public streaming

**Decision.** Secure pairing happens before public remote screen streaming.

**Status.** **Security requirement**

**Choice.** M1 pairing; M2 authenticated video. No unauthenticated production/debug remote stream in product builds. M0 is local-only.

**Alternatives.** Demo video first, wrap pairing later.

**Tradeoff.** Slower first “wow”; prevents leftover world-readable pipes.

**Blocks.** M1 before M2.

---

### D12. QR pairing properties

**Decision.** QR pairing is short-lived, one-time, explicitly approved, replay resistant, and session-bound.

**Status.** **Security requirement**

**Choice.** Server-side challenge; consume; TTL; owner approval required. No permanent password in the QR.

**Alternatives.** Reusable room codes; passwords in the QR.

**Tradeoff.** Slightly more UX friction; stops screenshot reuse.

**Blocks.** M1. Exact TTL seconds and consume-on-scan vs lock-on-scan/burn-on-accept still have a recommended default (see pairing protocol) pending lock.

---

### D13. SAS / DTLS fingerprint binding

**Decision.** Pairing will use an SAS/PIN on both endpoints and bind authenticated WebRTC peer identity / DTLS fingerprints.

**Status.** **Security requirement**

**Choice.** Two-phase identity: pairing SAS at approval (M1); fingerprint binding before media is `CONNECTED` (M2). See pairing protocol.

**Alternatives.** Trust coordinator TLS only; full user accounts.

**Tradeoff.** Extra UX step or extra confirmation; actually defends against coordinator MITM.

**Blocks.** M1 (SAS display architecture), M2 (fingerprint bind before frames). Exact SAS derivation is unresolved.

---

### D14. Coordinator origin allowlist

**Decision.** Android accepts only trusted/allowlisted coordinator origins.

**Status.** **Security requirement**

**Choice.** QR cannot redirect the app to an arbitrary URL.

**Alternatives.** Follow any `https://` in the QR.

**Tradeoff.** Operational need to ship origin updates vs phishing resistance.

**Blocks.** M1.

---

### D15. Capability set

**Decision.** Explicit capabilities: `screen.read`, `input.control`, `audio.read`. Android enforces each locally.

**Status.** **Invariant** (model) · **MVP decision** (initial set)

**Choice.** Start with these three names. Audio remains outside early milestones. Call audio stays out of scope.

**Alternatives.** Boolean “full control”; many fine-grained caps immediately.

**Tradeoff.** Enough for consent UI now; M5 granular UX can subdivide later without changing the PEP idea.

**Blocks.** M1 grant model.

---

### D16. Protected-screen control

**Decision.** Control into protected/black screen states is refused by default.

**Status.** **Security requirement**

**Choice.** If MediaProjection is black/`FLAG_SECURE`, do not inject.

**Alternatives.** Blind inject; a11y tree scrape (forbidden).

**Tradeoff.** Some banking support flows cannot be remote-controlled — correctly.

**Blocks.** M3.

---

### D17. Text input promise

**Decision.** Universal keyboard/HID emulation is not promised. Later text input is supported Unicode insertion into compatible focused fields.

**Status.** **MVP decision** · **Platform limitation**

**Choice.** M4 `text_input` with visible failure.

**Alternatives.** KeyEvent HID emulation as a marketing claim.

**Tradeoff.** Honest CJK/WebView failures vs fake keyboard.

**Blocks.** M4 copy and protocol, not M0–M3.

---

### D18. Min Android API

**Decision.** API 29 / Android 10.

**Status.** **MVP decision** · **Platform limitation**

**Choice.** 29 as floor; target current Play SDK when the Android module exists.

**Alternatives.** API 26 (painful projection/FGS history); API 31+ (drops more devices).

**Blocks.** M0.

---

### D19. Video codec

**Decision.** VP8 first; H.264 later only after real-device measurement.

**Status.** **MVP decision**

**Choice.** Interoperability over battery until measured.

**Alternatives.** H.264 hardware encode first.

**Blocks.** Soft for M2 encoder work; do not silently switch.

---

### D20. Control protocol encoding

**Decision.** Versioned JSON envelope on the DataChannel.

**Status.** **MVP decision**

**Choice.** JSON for three-language iteration. Protobuf later if needed.

**Alternatives.** Protobuf now; raw binary.

**Blocks.** M3 schema, not M0–M2.

---

### D21. Control coordinates

**Decision.** Normalized `[0,1]` relative to the encoded video frame.

**Status.** **MVP decision**

**Choice.** Android maps frame space → display pixels, including rotation and letterbox.

**Alternatives.** Raw Android pixels; CSS canvas pixels as protocol truth.

**Tradeoff.** Mapping code on Android; avoids browser/layout bugs as protocol.

**Blocks.** M3.

---

### D22. Audio scope

**Decision.** Audio is outside early MVP. Call audio is out of scope.

**Status.** **MVP decision** (timing) · **Invariant** (no call audio)

**Choice.** M5 optional supported playback/mic if still justified. Never in-call capture.

**Blocks.** Forbids audio work in M0–M4.

---

### D23. Session termination and TTL

**Decision.** Closed/expired sessions cannot silently reopen. Process death, reboot, or lost MediaProjection never silently restore capture. Hard max TTL 60 minutes, configurable later.

**Status.** **Invariant** (termination) · **MVP decision** (60 minutes)

**Choice.** Terminal states + no boot persistence + hard stop.

**Alternatives.** Until disconnect only; unattended reconnect.

**Blocks.** M1 lifecycle; M4 death UX.

---

### D24. Implementation milestone order

**Decision.** M0 capture foundation → M1 secure session → M2 authenticated viewing with TURN → M3 supported control → M4 reliability/text → M5 hardening.

**Status.** **MVP decision**

**Choice.** Pairing before public streaming; TURN inside M2; control after viewing.

**Alternatives.** Video demo first; TURN after control.

**Tradeoff.** Less demo-friendly; safer and geographically honest.

**Blocks.** Scheduling of all implementation. Do not start M2 code until M1 session foundation exists. Do not start M3 until M2 viewing exists.

---

## Open decisions (do not block M0)

These are **not** locked. Documented so they are not silently “decided in code.”

| ID | Topic | Recommendation | Blocks |
|---|---|---|---|
| O1 | QR TTL duration | 60–120 seconds | M1 |
| O2 | Session TTL anchor | Start at owner approve, not at QR create | M1 |
| O3 | Pairing consume semantics | Exclusive lock on scan; burn on accept or approval timeout | M1 |
| O4 | SAS derivation | HKDF over pairing transcript; later mix DTLS fingerprints | M1/M2 |
| O5 | SAS UX | 6-digit groups shown on both screens | M1 |
| O6 | SDP offerer | Phone offers, browser answers | M2 |
| O7 | Operator tab refresh | One WebRTC re-handshake on same `sid` while projection lives | M2 |
| O8 | TURN provider/region | Must be reachable from China and Bangladesh including 443 | M2 |
| O9 | ICE policy | `all` (prefer direct) with force-relay test mode | M2 |
| O10 | Local agent port/auth | `127.0.0.1` + unguessable token; port TBD | Agent milestone |
| O11 | Debug M0 preview channel | Compile-time flag only; never a product flavor | M0 |
| O12 | Play Integrity / device attestation | Not in MVP | Future |
| O13 | Protocol IDL location | Docs now; generated schemas when M1/M3 start | M1/M3 |

O11 should be chosen when M0 starts so a LAN URL cannot leak into release builds.
