package com.phonebeam.android.pairing

import java.net.URI

data class PairingQr(
    val version: Int,
    val origin: String,
    val sessionId: String,
    val pairingId: String,
    val expiresAtEpochSeconds: Long,
)

enum class PairingQrReject {
    MALFORMED,
    UNKNOWN_VERSION,
    UNTRUSTED_ORIGIN,
    EXPIRED,
}

sealed class PairingQrParse {
    data class Ok(val payload: PairingQr) : PairingQrParse()
    data class Rejected(val reason: PairingQrReject) : PairingQrParse()
}

object PairingQrParser {
    const val PROTOCOL_VERSION = 1
    private const val MAX_BYTES = 4096
    private val requiredKeys = setOf("v", "origin", "sid", "pid", "exp")

    fun parse(raw: String, nowEpochSeconds: Long): PairingQrParse {
        val trimmed = raw.trim()
        if (trimmed.isEmpty() || trimmed.length > MAX_BYTES) {
            return PairingQrParse.Rejected(PairingQrReject.MALFORMED)
        }
        if (looksLikeNavigableUri(trimmed)) {
            return PairingQrParse.Rejected(PairingQrReject.MALFORMED)
        }
        val fields = parseFlatObject(trimmed) ?: return PairingQrParse.Rejected(PairingQrReject.MALFORMED)
        if (fields.keys != requiredKeys) {
            return PairingQrParse.Rejected(PairingQrReject.MALFORMED)
        }
        val version = fields["v"]?.toIntOrNull() ?: return PairingQrParse.Rejected(PairingQrReject.MALFORMED)
        if (version != PROTOCOL_VERSION) {
            return PairingQrParse.Rejected(PairingQrReject.UNKNOWN_VERSION)
        }
        val origin = fields.getValue("origin")
        val sid = fields.getValue("sid")
        val pid = fields.getValue("pid")
        val exp = fields["exp"]?.toLongOrNull() ?: return PairingQrParse.Rejected(PairingQrReject.MALFORMED)
        if (origin.isBlank() || sid.isBlank() || pid.isBlank() || exp <= 0L) {
            return PairingQrParse.Rejected(PairingQrReject.MALFORMED)
        }
        if (!isBareHttpOrigin(origin) || !TrustedCoordinators.isTrusted(origin)) {
            return PairingQrParse.Rejected(PairingQrReject.UNTRUSTED_ORIGIN)
        }
        if (nowEpochSeconds >= exp) {
            return PairingQrParse.Rejected(PairingQrReject.EXPIRED)
        }
        return PairingQrParse.Ok(
            PairingQr(
                version = version,
                origin = TrustedCoordinators.normalize(origin),
                sessionId = sid,
                pairingId = pid,
                expiresAtEpochSeconds = exp,
            ),
        )
    }

    private fun looksLikeNavigableUri(raw: String): Boolean {
        val lower = raw.lowercase()
        return lower.startsWith("http://") ||
            lower.startsWith("https://") ||
            lower.startsWith("intent:") ||
            lower.startsWith("javascript:") ||
            lower.startsWith("market:")
    }

    private fun isBareHttpOrigin(origin: String): Boolean {
        val uri = try {
            URI(origin)
        } catch (_: Exception) {
            return false
        }
        if (uri.scheme != "http" && uri.scheme != "https") {
            return false
        }
        if (uri.host.isNullOrBlank() || !uri.userInfo.isNullOrBlank()) {
            return false
        }
        if (!uri.path.isNullOrEmpty() && uri.path != "/") {
            return false
        }
        if (!uri.query.isNullOrBlank() || !uri.fragment.isNullOrBlank()) {
            return false
        }
        return true
    }

    /** Strict flat JSON object: quoted keys, string or integer values, no nesting. */
    internal fun parseFlatObject(raw: String): Map<String, String>? {
        if (!raw.startsWith("{") || !raw.endsWith("}")) {
            return null
        }
        val body = raw.substring(1, raw.length - 1).trim()
        if (body.isEmpty()) {
            return emptyMap()
        }
        val out = linkedMapOf<String, String>()
        var i = 0
        while (i < body.length) {
            while (i < body.length && body[i].isWhitespace()) i++
            val key = readJsonString(body, i) ?: return null
            i = key.end
            while (i < body.length && body[i].isWhitespace()) i++
            if (i >= body.length || body[i] != ':') {
                return null
            }
            i++
            while (i < body.length && body[i].isWhitespace()) i++
            val value = readJsonValue(body, i) ?: return null
            if (out.put(key.value, value.value) != null) {
                return null
            }
            i = value.end
            while (i < body.length && body[i].isWhitespace()) i++
            if (i == body.length) {
                break
            }
            if (body[i] != ',') {
                return null
            }
            i++
        }
        return out
    }

    private data class Token(val value: String, val end: Int)

    private fun readJsonString(source: String, start: Int): Token? {
        if (start >= source.length || source[start] != '"') {
            return null
        }
        val out = StringBuilder()
        var i = start + 1
        while (i < source.length) {
            val ch = source[i]
            if (ch == '"') {
                return Token(out.toString(), i + 1)
            }
            if (ch == '\\') {
                return null
            }
            if (ch.code < 32) {
                return null
            }
            out.append(ch)
            i++
        }
        return null
    }

    private fun readJsonValue(source: String, start: Int): Token? {
        if (start >= source.length) {
            return null
        }
        if (source[start] == '"') {
            return readJsonString(source, start)
        }
        var i = start
        if (source[i] == '-') {
            i++
        }
        if (i >= source.length || !source[i].isDigit()) {
            return null
        }
        while (i < source.length && source[i].isDigit()) {
            i++
        }
        return Token(source.substring(start, i), i)
    }
}
