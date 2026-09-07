package com.phonebeam.android.control

import android.app.KeyguardManager
import android.content.Context
import org.json.JSONArray
import org.json.JSONObject
import org.webrtc.DataChannel
import org.webrtc.PeerConnection
import org.webrtc.VideoTrack
import java.nio.ByteBuffer
import java.nio.charset.StandardCharsets

class ControlSession(
    private val context: Context,
    private val sessionId: String,
    private val effectiveCaps: List<String>,
    private val onHangup: () -> Unit,
) {
    private var channel: DataChannel? = null
    private var outboundSeq = 1L
    private var lastInboundSeq = 0L
    private var coordinatorState = ""
    private var frameWidth = 0
    private var frameHeight = 0
    private var projectionLive = true
    private var gestureInFlight = false
    private val recent = ArrayDeque<Long>()
    private val probe = BlackFrameProbe()
    private var videoTrack: VideoTrack? = null
    private var lastLiveCaps: List<String> = emptyList()

    fun attach(pc: PeerConnection, track: VideoTrack, width: Int, height: Int) {
        frameWidth = width
        frameHeight = height
        videoTrack = track
        track.addSink(probe)
        val init = DataChannel.Init().apply { ordered = true }
        val dc = pc.createDataChannel(ControlProtocol.CHANNEL, init)
        channel = dc
        dc.registerObserver(object : DataChannel.Observer {
            override fun onBufferedAmountChange(previousAmount: Long) = Unit
            override fun onStateChange() {
                if (dc.state() == DataChannel.State.OPEN) {
                    sendCapabilityUpdate()
                }
            }
            override fun onMessage(buffer: DataChannel.Buffer?) {
                if (buffer == null || buffer.binary) {
                    return
                }
                val bytes = ByteArray(buffer.data.remaining())
                buffer.data.get(bytes)
                handleRaw(String(bytes, StandardCharsets.UTF_8))
            }
        })
    }

    fun onCoordinatorState(state: String) {
        val previous = coordinatorState
        coordinatorState = state
        if (state == "CONNECTED" && previous != "CONNECTED") {
            lastLiveCaps = emptyList()
            sendCapabilityUpdate()
        }
    }

    fun onProjectionLive(live: Boolean) {
        projectionLive = live
    }

    fun onFrameSize(width: Int, height: Int) {
        if (width > 0 && height > 0) {
            frameWidth = width
            frameHeight = height
        }
    }

    fun onAccessibilityChanged() {
        sendCapabilityUpdate()
    }

    private fun handleRaw(raw: String) {
        val env = try {
            ControlProtocol.parse(raw)
        } catch (err: ControlParseException) {
            sendError(0, err.code)
            return
        }
        val now = System.currentTimeMillis()
        prune(now)
        val ctx = PepContext(
            sessionId = sessionId,
            sessionState = coordinatorState,
            effectiveCaps = effectiveCaps,
            accessibilityEnabled = PhoneBeamAccessibilityService.isRunning(),
            projectionLive = projectionLive,
            keyguardLocked = keyguardLocked(),
            protectedContent = probe.isProtected() || !probe.hasFrame(),
            lastSeq = lastInboundSeq,
            gestureInFlight = gestureInFlight,
            commandsInLastSecond = recent.size,
            frameWidth = frameWidth,
            frameHeight = frameHeight,
        )
        val decision = CommandPep.evaluate(env, ctx)
        if (!decision.ok) {
            sendError(env.seq, decision.code)
            return
        }
        lastInboundSeq = env.seq
        recent.addLast(now)
        when (decision.execute) {
            "ping" -> sendPong(env.seq)
            "session_close" -> {
                sendAck(env.seq)
                onHangup()
            }
            "tap" -> runGesture(env) { svc ->
                val p = Geometry.mapNormalized(env.body.getDouble("x"), env.body.getDouble("y"), frameWidth, frameHeight)
                    ?: return@runGesture false
                svc.tap(p.x, p.y)
            }
            "swipe" -> runGesture(env) { svc ->
                val a = Geometry.mapNormalized(env.body.getDouble("x1"), env.body.getDouble("y1"), frameWidth, frameHeight)
                val b = Geometry.mapNormalized(env.body.getDouble("x2"), env.body.getDouble("y2"), frameWidth, frameHeight)
                if (a == null || b == null) return@runGesture false
                svc.swipe(a.x, a.y, b.x, b.y, env.body.getLong("duration_ms"))
            }
            "back" -> runGesture(env) { it.back() }
            "home" -> runGesture(env) { it.home() }
        }
    }

    private fun runGesture(env: ControlEnvelope, action: (PhoneBeamAccessibilityService) -> Boolean) {
        val svc = PhoneBeamAccessibilityService.instance
        if (svc == null) {
            sendError(env.seq, "unsupported_on_device")
            sendCapabilityUpdate()
            return
        }
        gestureInFlight = true
        val ok = try {
            action(svc)
        } catch (_: Exception) {
            false
        } finally {
            gestureInFlight = false
        }
        if (ok) sendAck(env.seq) else sendError(env.seq, "unsupported_on_device")
    }

    private fun sendCapabilityUpdate() {
        val live = CommandPep.liveControlCaps(
            effectiveCaps,
            PhoneBeamAccessibilityService.isRunning(),
            probe.isProtected() || keyguardLocked(),
        )
        if (live == lastLiveCaps && lastLiveCaps.isNotEmpty()) {
            return
        }
        lastLiveCaps = live
        (context.applicationContext as? com.phonebeam.android.PhoneBeamApp)?.liveControlCaps = live
        val body = JSONObject().put("effective", JSONArray(live))
        send(
            ControlEnvelope(
                v = ControlProtocol.VERSION,
                sid = sessionId,
                seq = nextSeq(),
                ts = System.currentTimeMillis(),
                cap = "",
                type = "capability_update",
                body = body,
            ),
        )
    }

    private fun sendAck(ref: Long) {
        send(
            ControlEnvelope(
                v = ControlProtocol.VERSION,
                sid = sessionId,
                seq = nextSeq(),
                ts = System.currentTimeMillis(),
                cap = "",
                type = "ack",
                body = JSONObject().put("ref", ref),
            ),
        )
    }

    private fun sendError(ref: Long, code: String) {
        send(
            ControlEnvelope(
                v = ControlProtocol.VERSION,
                sid = sessionId,
                seq = nextSeq(),
                ts = System.currentTimeMillis(),
                cap = "",
                type = "error",
                body = JSONObject().put("ref", ref).put("code", code),
            ),
        )
    }

    private fun sendPong(ref: Long) {
        send(
            ControlEnvelope(
                v = ControlProtocol.VERSION,
                sid = sessionId,
                seq = nextSeq(),
                ts = System.currentTimeMillis(),
                cap = "",
                type = "pong",
                body = JSONObject().put("ref", ref),
            ),
        )
    }

    private fun send(env: ControlEnvelope) {
        val dc = channel ?: return
        if (dc.state() != DataChannel.State.OPEN) {
            return
        }
        val bytes = env.encode().toByteArray(StandardCharsets.UTF_8)
        dc.send(DataChannel.Buffer(ByteBuffer.wrap(bytes), false))
    }

    private fun nextSeq(): Long {
        val n = outboundSeq
        outboundSeq += 1
        return n
    }

    private fun prune(now: Long) {
        while (recent.isNotEmpty() && now - recent.first() > 1000) {
            recent.removeFirst()
        }
    }

    private fun keyguardLocked(): Boolean {
        val km = context.getSystemService(Context.KEYGUARD_SERVICE) as KeyguardManager
        return km.isKeyguardLocked
    }

    fun stop() {
        videoTrack?.removeSink(probe)
        videoTrack = null
        channel?.unregisterObserver()
        channel?.close()
        channel?.dispose()
        channel = null
    }
}
