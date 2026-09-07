package com.phonebeam.android.webrtc

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ViewingPolicyTest {
    @Test
    fun remoteShareRequiresCapsBoundAndScreenRead() {
        assertTrue(ViewingPolicy.canStartRemoteShare("CAPS_BOUND", listOf("screen.read")))
        assertTrue(ViewingPolicy.canStartRemoteShare("PROJECTION_DENIED", listOf("screen.read")))
        assertFalse(ViewingPolicy.canStartRemoteShare("APPROVAL_PENDING", listOf("screen.read")))
        assertFalse(ViewingPolicy.canStartRemoteShare("CAPS_BOUND", listOf("input.control")))
        assertFalse(ViewingPolicy.iceAllowsPeerReady("CHECKING"))
        assertTrue(ViewingPolicy.iceAllowsPeerReady("CONNECTED"))
        assertTrue(ViewingPolicy.iceAllowsPeerReady("COMPLETED"))
    }

    @Test
    fun reconnectIsBoundedAndProjectionBound() {
        assertTrue(ViewingPolicy.reconnectEligible(true, false, false, true))
        assertFalse(ViewingPolicy.reconnectEligible(false, false, false, true))
        assertFalse(ViewingPolicy.reconnectEligible(true, true, false, true))
        assertFalse(ViewingPolicy.reconnectEligible(true, false, true, true))
        assertFalse(ViewingPolicy.reconnectEligible(true, false, false, false))
    }

    @Test
    fun projectionRevokeEndsSession() {
        assertEquals("CLOSED", ViewingPolicy.afterProjectionRevoked())
    }

    @Test
    fun signalingValidation() {
        assertTrue(ViewingPolicy.signalingVersionOk(1))
        assertFalse(ViewingPolicy.signalingVersionOk(2))
        assertTrue(ViewingPolicy.validCandidate("candidate:1 1 UDP 1 127.0.0.1 9 typ host"))
        assertFalse(ViewingPolicy.validCandidate("turn:evil"))
        assertFalse(ViewingPolicy.iceServersFromPeerSignalingAllowed())
    }
}
