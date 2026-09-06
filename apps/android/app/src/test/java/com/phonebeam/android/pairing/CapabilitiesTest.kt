package com.phonebeam.android.pairing

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class CapabilitiesTest {

    @Test
    fun intersectionIsNotUnion() {
        val effective = Capabilities.intersect(
            requested = listOf("screen.read", "input.control", "audio.read"),
            granted = listOf("screen.read", "input.control"),
            device = listOf("screen.read"),
        )
        assertEquals(listOf("screen.read"), effective)
    }

    @Test
    fun cannotTreatUnrequestedAsGrantableSubset() {
        assertFalse(Capabilities.subset(listOf("audio.read"), listOf("screen.read")))
        assertTrue(Capabilities.subset(listOf("screen.read"), listOf("screen.read", "input.control")))
    }

    @Test
    fun m1DeviceAvailabilityExcludesControlAndAudio() {
        assertEquals(listOf("screen.read"), Capabilities.deviceAvailable())
        assertFalse("input.control" in Capabilities.deviceAvailable())
        assertFalse("audio.read" in Capabilities.deviceAvailable())
    }
}
