package com.phonebeam.android.webrtc

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SasTest {
    @Test
    fun goldenMediaVectorMatchesCoordinator() {
        val key = "phonebeam-sas-test-key-32bytes!!".toByteArray()
        val got = Sas.mediaDisplay(
            key,
            "http://127.0.0.1:8080",
            "sid-a",
            "pid-b",
            "Support-A",
            listOf("input.control", "screen.read"),
            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
            "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
        )
        assertEquals("830 034", got)
    }

    @Test
    fun mediaSasDiffersFromPairingSas() {
        val key = ByteArray(32) { 1 }
        val pairing = Sas.pairingDisplay(key, "http://127.0.0.1:8080", "s", "p", "Op", listOf("screen.read"))
        val media = Sas.mediaDisplay(
            key, "http://127.0.0.1:8080", "s", "p", "Op", listOf("screen.read"),
            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
            "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
        )
        assertNotEquals(pairing, media)
        assertTrue(Sas.validSha256Fingerprint("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
    }

    @Test
    fun extractFingerprintCanonicalizes() {
        val sdp = "v=0\na=fingerprint:sha-256 AB:CD:EF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:01:23:45:67:89:AB:CD:EF:00:11:22:33:44\n"
        val fp = ViewingRuntime.extractFingerprint(sdp)
        assertEquals("abcdef00112233445566778899aabbccddeeff0123456789abcdef0011223344", fp)
    }
}
