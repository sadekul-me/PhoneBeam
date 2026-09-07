package com.phonebeam.android.control

import org.json.JSONObject

data class ControlEnvelope(
    val v: Int,
    val sid: String,
    val seq: Long,
    val ts: Long,
    val cap: String,
    val type: String,
    val body: JSONObject,
) {
    fun encode(): String {
        return JSONObject()
            .put("v", v)
            .put("sid", sid)
            .put("seq", seq)
            .put("ts", ts)
            .put("cap", cap)
            .put("type", type)
            .put("body", body)
            .toString()
    }
}

object ControlProtocol {
    const val VERSION = 1
    const val MAX_BYTES = 8192
    const val CHANNEL = "phonebeam-control"
    const val GOLDEN_TAP =
        """{"v":1,"sid":"sid-a","seq":7,"ts":1700000000000,"cap":"input.control","type":"tap","body":{"x":0.25,"y":0.75}}"""

    private val topKeys = setOf("v", "sid", "seq", "ts", "cap", "type", "body")
    private val knownTypes = setOf(
        "tap", "swipe", "back", "home", "ping", "pong",
        "capability_update", "session_close", "ack", "error",
    )

    fun parse(raw: String): ControlEnvelope {
        if (raw.isEmpty() || raw.length > MAX_BYTES) {
            throw ControlParseException("invalid_body")
        }
        val obj = try {
            JSONObject(raw)
        } catch (_: Exception) {
            throw ControlParseException("invalid_body")
        }
        val keys = obj.keys().asSequence().toSet()
        if (!keys.containsAll(setOf("v", "sid", "seq", "ts", "type")) || keys.any { it !in topKeys }) {
            throw ControlParseException("invalid_body")
        }
        val v = obj.getInt("v")
        if (v != VERSION) {
            throw ControlParseException("unsupported_version")
        }
        val type = obj.getString("type")
        if (type !in knownTypes) {
            throw ControlParseException("unknown_type")
        }
        val body = obj.optJSONObject("body") ?: JSONObject()
        return ControlEnvelope(
            v = v,
            sid = obj.getString("sid"),
            seq = obj.getLong("seq"),
            ts = obj.getLong("ts"),
            cap = obj.optString("cap"),
            type = type,
            body = body,
        )
    }
}

class ControlParseException(val code: String) : Exception(code)
