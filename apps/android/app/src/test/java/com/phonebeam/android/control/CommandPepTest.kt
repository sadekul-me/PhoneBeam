package com.phonebeam.android.control

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.json.JSONObject

class CommandPepTest {
    private val connected = PepContext(
        sessionId = "sid-a",
        sessionState = "CONNECTED",
        effectiveCaps = listOf("screen.read", "input.control"),
        accessibilityEnabled = true,
        projectionLive = true,
        keyguardLocked = false,
        protectedContent = false,
        lastSeq = 6,
        gestureInFlight = false,
        commandsInLastSecond = 0,
        frameWidth = 1080,
        frameHeight = 2400,
    )

    @Test
    fun goldenTapParses() {
        val env = ControlProtocol.parse(ControlProtocol.GOLDEN_TAP)
        assertEquals(1, env.v)
        assertEquals("sid-a", env.sid)
        assertEquals(7L, env.seq)
        assertEquals("tap", env.type)
        assertEquals(0.25, env.body.getDouble("x"), 0.0)
    }

    @Test
    fun versionAndUnknownType() {
        try {
            ControlProtocol.parse("""{"v":2,"sid":"s","seq":1,"ts":1,"cap":"input.control","type":"tap","body":{}}""")
            throw AssertionError("expected version reject")
        } catch (err: ControlParseException) {
            assertEquals("unsupported_version", err.code)
        }
        try {
            ControlProtocol.parse("""{"v":1,"sid":"s","seq":1,"ts":1,"cap":"input.control","type":"unlock","body":{}}""")
            throw AssertionError("expected unknown type")
        } catch (err: ControlParseException) {
            assertEquals("unknown_type", err.code)
        }
    }

    @Test
    fun sidMismatchAndReplay() {
        val tap = ControlProtocol.parse(ControlProtocol.GOLDEN_TAP)
        assertEquals("session_inactive", CommandPep.evaluate(tap, connected.copy(sessionId = "other")).code)
        assertEquals("replay", CommandPep.evaluate(tap, connected.copy(lastSeq = 7)).code)
    }

    @Test
    fun missingCapAndRevokedAndNoA11y() {
        val tap = ControlProtocol.parse(ControlProtocol.GOLDEN_TAP)
        assertEquals("capability_denied", CommandPep.evaluate(tap, connected.copy(effectiveCaps = listOf("screen.read"))).code)
        assertEquals("unsupported_on_device", CommandPep.evaluate(tap, connected.copy(accessibilityEnabled = false)).code)
    }

    @Test
    fun protectedAndKeyguardRefused() {
        val tap = ControlProtocol.parse(ControlProtocol.GOLDEN_TAP)
        assertEquals("protected_content", CommandPep.evaluate(tap, connected.copy(protectedContent = true)).code)
        assertEquals("protected_content", CommandPep.evaluate(tap, connected.copy(keyguardLocked = true)).code)
    }

    @Test
    fun invalidCoordinatesAndRateLimit() {
        val bad = ControlEnvelope(1, "sid-a", 8, 1, "input.control", "tap", JSONObject().put("x", 1.5).put("y", 0.2))
        assertEquals("unsupported_geometry", CommandPep.evaluate(bad, connected).code)
        val tap = ControlProtocol.parse(ControlProtocol.GOLDEN_TAP)
        assertEquals("rate_limited", CommandPep.evaluate(tap, connected.copy(gestureInFlight = true)).code)
        assertEquals("rate_limited", CommandPep.evaluate(tap, connected.copy(commandsInLastSecond = 8)).code)
    }

    @Test
    fun notConnectedRejectsGesturesButPingOk() {
        val tap = ControlProtocol.parse(ControlProtocol.GOLDEN_TAP)
        assertEquals("session_inactive", CommandPep.evaluate(tap, connected.copy(sessionState = "NEGOTIATING")).code)
        val ping = ControlEnvelope(1, "sid-a", 8, 1, "", "ping", JSONObject())
        assertTrue(CommandPep.evaluate(ping, connected.copy(sessionState = "RECONNECTING")).ok)
    }

    @Test
    fun capabilityUpdateDropsControlWhenA11yOff() {
        val live = CommandPep.liveControlCaps(listOf("screen.read", "input.control"), false, false)
        assertEquals(listOf("screen.read"), live)
        assertFalse("input.control" in live)
    }

    @Test
    fun geometryMapsAndRejects() {
        assertNotNull(Geometry.mapNormalized(0.0, 0.0, 100, 200))
        assertNotNull(Geometry.mapNormalized(1.0, 1.0, 100, 200))
        assertNull(Geometry.mapNormalized(-0.01, 0.5, 100, 200))
        assertNull(Geometry.mapNormalized(0.5, 0.5, 0, 200))
    }

    @Test
    fun happyTap() {
        val tap = ControlProtocol.parse(ControlProtocol.GOLDEN_TAP)
        val decision = CommandPep.evaluate(tap, connected)
        assertTrue(decision.ok)
        assertEquals("tap", decision.execute)
    }
}
