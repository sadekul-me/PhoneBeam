package com.phonebeam.android.webrtc

object ViewingPolicy {
    fun canStartRemoteShare(sessionState: String, effectiveCaps: List<String>): Boolean {
        return (sessionState == "CAPS_BOUND" || sessionState == "PROJECTION_DENIED") &&
            "screen.read" in effectiveCaps
    }

    fun iceAllowsPeerReady(iceState: String): Boolean {
        return iceState.equals("CONNECTED", ignoreCase = true) ||
            iceState.equals("COMPLETED", ignoreCase = true)
    }

    fun reconnectEligible(
        projectionLive: Boolean,
        rehandshakeUsed: Boolean,
        ttlExpired: Boolean,
        sameOperator: Boolean,
    ): Boolean {
        return projectionLive && !rehandshakeUsed && !ttlExpired && sameOperator
    }

    fun afterProjectionRevoked(): String = "CLOSED"

    fun iceServersFromPeerSignalingAllowed(): Boolean = false

    fun signalingVersionOk(version: Int): Boolean = version == 1

    fun validCandidate(line: String): Boolean {
        val trimmed = line.trim()
        return trimmed.startsWith("candidate:") || trimmed.startsWith("a=candidate:")
    }
}
