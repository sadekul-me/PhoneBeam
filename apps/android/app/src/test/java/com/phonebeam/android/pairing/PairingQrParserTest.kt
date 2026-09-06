package com.phonebeam.android.pairing

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class PairingQrParserTest {

    private val valid = """{"v":1,"origin":"http://10.0.2.2:8080","sid":"sid1","pid":"pid1","exp":1700000120}"""

    @Test
    fun acceptsPhoneBeamPayload() {
        val parsed = PairingQrParser.parse(valid, nowEpochSeconds = 1_700_000_000L)
        val ok = parsed as PairingQrParse.Ok
        assertEquals("sid1", ok.payload.sessionId)
        assertEquals("http://10.0.2.2:8080", ok.payload.origin)
    }

    @Test
    fun rejectsMalformedAndUrls() {
        val now = 1_700_000_000L
        val cases = listOf(
            "",
            "not-json",
            "https://evil.example/join",
            "intent://scan#Intent;end",
            """{"v":1,"origin":"http://10.0.2.2:8080","sid":"sid1","exp":1700000120}""",
        )
        cases.forEach { raw ->
            val parsed = PairingQrParser.parse(raw, now)
            assertTrue(raw, parsed is PairingQrParse.Rejected)
            assertEquals(raw, PairingQrReject.MALFORMED, (parsed as PairingQrParse.Rejected).reason)
        }
    }

    @Test
    fun rejectsUnknownVersion() {
        val raw = """{"v":2,"origin":"http://10.0.2.2:8080","sid":"sid1","pid":"pid1","exp":1700000120}"""
        val parsed = PairingQrParser.parse(raw, 1_700_000_000L) as PairingQrParse.Rejected
        assertEquals(PairingQrReject.UNKNOWN_VERSION, parsed.reason)
    }

    @Test
    fun rejectsUntrustedOrigin() {
        val raw = """{"v":1,"origin":"https://evil.example","sid":"sid1","pid":"pid1","exp":1700000120}"""
        val parsed = PairingQrParser.parse(raw, 1_700_000_000L) as PairingQrParse.Rejected
        assertEquals(PairingQrReject.UNTRUSTED_ORIGIN, parsed.reason)
    }

    @Test
    fun rejectsExpiredPayload() {
        val parsed = PairingQrParser.parse(valid, nowEpochSeconds = 1_700_000_120L) as PairingQrParse.Rejected
        assertEquals(PairingQrReject.EXPIRED, parsed.reason)
    }
}
