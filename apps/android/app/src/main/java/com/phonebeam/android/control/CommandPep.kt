package com.phonebeam.android.control

data class PepContext(
    val sessionId: String,
    val sessionState: String,
    val effectiveCaps: List<String>,
    val accessibilityEnabled: Boolean,
    val projectionLive: Boolean,
    val keyguardLocked: Boolean,
    val protectedContent: Boolean,
    val lastSeq: Long,
    val gestureInFlight: Boolean,
    val commandsInLastSecond: Int,
    val frameWidth: Int,
    val frameHeight: Int,
)

data class PepDecision(
    val ok: Boolean,
    val code: String = "",
    val execute: String = "",
)

object CommandPep {
    const val MAX_PER_SECOND = 8
    const val MIN_SWIPE_MS = 50L
    const val MAX_SWIPE_MS = 2000L

    fun evaluate(env: ControlEnvelope, ctx: PepContext): PepDecision {
        if (env.sid != ctx.sessionId) {
            return deny("session_inactive")
        }
        if (env.seq <= ctx.lastSeq) {
            return deny("replay")
        }
        return when (env.type) {
            "ping", "session_close" -> evaluateSessionControl(env, ctx)
            "tap", "swipe", "back", "home" -> evaluateInput(env, ctx)
            "ack", "error", "pong", "capability_update" -> deny("unknown_type")
            else -> deny("unknown_type")
        }
    }

    private fun evaluateSessionControl(env: ControlEnvelope, ctx: PepContext): PepDecision {
        if (ctx.sessionState == "CLOSED" || ctx.sessionState == "EXPIRED" || ctx.sessionState == "REJECTED") {
            return deny("session_inactive")
        }
        if (env.type == "ping") {
            return PepDecision(ok = true, execute = "ping")
        }
        return PepDecision(ok = true, execute = "session_close")
    }

    private fun evaluateInput(env: ControlEnvelope, ctx: PepContext): PepDecision {
        if (ctx.sessionState != "CONNECTED") {
            return deny("session_inactive")
        }
        if (!ctx.projectionLive) {
            return deny("session_inactive")
        }
        if (env.cap != "input.control") {
            return deny("capability_denied")
        }
        if ("input.control" !in ctx.effectiveCaps) {
            return deny("capability_denied")
        }
        if (!ctx.accessibilityEnabled) {
            return deny("unsupported_on_device")
        }
        if (ctx.keyguardLocked || ctx.protectedContent) {
            return deny("protected_content")
        }
        if (ctx.gestureInFlight && env.type in setOf("tap", "swipe")) {
            return deny("rate_limited")
        }
        if (ctx.commandsInLastSecond >= MAX_PER_SECOND) {
            return deny("rate_limited")
        }
        when (env.type) {
            "tap" -> {
                val x = env.body.optDouble("x", Double.NaN)
                val y = env.body.optDouble("y", Double.NaN)
                if (Geometry.mapNormalized(x, y, ctx.frameWidth, ctx.frameHeight) == null) {
                    return deny("unsupported_geometry")
                }
            }
            "swipe" -> {
                val x1 = env.body.optDouble("x1", Double.NaN)
                val y1 = env.body.optDouble("y1", Double.NaN)
                val x2 = env.body.optDouble("x2", Double.NaN)
                val y2 = env.body.optDouble("y2", Double.NaN)
                val duration = env.body.optLong("duration_ms", 0)
                if (Geometry.mapNormalized(x1, y1, ctx.frameWidth, ctx.frameHeight) == null ||
                    Geometry.mapNormalized(x2, y2, ctx.frameWidth, ctx.frameHeight) == null
                ) {
                    return deny("unsupported_geometry")
                }
                if (duration < MIN_SWIPE_MS || duration > MAX_SWIPE_MS) {
                    return deny("invalid_body")
                }
            }
        }
        return PepDecision(ok = true, execute = env.type)
    }

    private fun deny(code: String) = PepDecision(ok = false, code = code)

    fun liveControlCaps(effective: List<String>, accessibilityEnabled: Boolean, protectedContent: Boolean): List<String> {
        return effective.filter { cap ->
            when (cap) {
                "input.control" -> accessibilityEnabled && !protectedContent
                else -> true
            }
        }
    }
}
