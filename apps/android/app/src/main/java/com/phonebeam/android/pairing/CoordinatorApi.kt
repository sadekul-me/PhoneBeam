package com.phonebeam.android.pairing

import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import java.nio.charset.StandardCharsets

data class RemoteSession(
    val id: String,
    val state: String,
    val operatorDisplayName: String,
    val requestedCapabilities: List<String>,
    val grantedCapabilities: List<String>,
    val deviceCapabilities: List<String>,
    val effectiveCapabilities: List<String>,
    val sas: String,
)

sealed class CoordinatorResult<out T> {
    data class Ok<T>(val value: T) : CoordinatorResult<T>()
    data class Err(val code: String, val message: String) : CoordinatorResult<Nothing>()
}

data class ScanResponse(
    val session: RemoteSession,
    val phonePendingToken: String,
)

data class ApproveResponse(
    val session: RemoteSession,
    val phoneToken: String,
)

class CoordinatorApi(private val origin: String) {
    fun scan(sessionId: String, pairingId: String): CoordinatorResult<ScanResponse> {
        val body = JSONObject().put("pid", pairingId).toString()
        return when (val raw = request("POST", "/api/v1/sessions/${enc(sessionId)}/scan", body, token = null)) {
            is CoordinatorResult.Err -> raw
            is CoordinatorResult.Ok -> {
                val session = parseSession(raw.value.optJSONObject("session") ?: return missing())
                val pending = raw.value.optString("phone_pending_token")
                if (pending.isBlank()) missing() else CoordinatorResult.Ok(ScanResponse(session, pending))
            }
        }
    }

    fun approve(
        sessionId: String,
        pendingToken: String,
        granted: List<String>,
        device: List<String>,
    ): CoordinatorResult<ApproveResponse> {
        val body = JSONObject()
            .put("granted_capabilities", JSONArray(granted))
            .put("device_capabilities", JSONArray(device))
            .toString()
        return when (val raw = request("POST", "/api/v1/sessions/${enc(sessionId)}/approve", body, pendingToken)) {
            is CoordinatorResult.Err -> raw
            is CoordinatorResult.Ok -> {
                val session = parseSession(raw.value.optJSONObject("session") ?: return missing())
                CoordinatorResult.Ok(ApproveResponse(session, raw.value.optString("phone_token")))
            }
        }
    }

    fun reject(sessionId: String, pendingToken: String): CoordinatorResult<RemoteSession> {
        return when (val raw = request("POST", "/api/v1/sessions/${enc(sessionId)}/reject", "{}", pendingToken)) {
            is CoordinatorResult.Err -> raw
            is CoordinatorResult.Ok -> {
                val session = parseSession(raw.value.optJSONObject("session") ?: return missing())
                CoordinatorResult.Ok(session)
            }
        }
    }

    fun get(sessionId: String, token: String): CoordinatorResult<RemoteSession> {
        return when (val raw = request("GET", "/api/v1/sessions/${enc(sessionId)}", body = null, token = token)) {
            is CoordinatorResult.Err -> raw
            is CoordinatorResult.Ok -> {
                val session = parseSession(raw.value.optJSONObject("session") ?: return missing())
                CoordinatorResult.Ok(session)
            }
        }
    }

    private fun missing(): CoordinatorResult.Err = CoordinatorResult.Err("invalid_json", "invalid coordinator response")

    private fun request(
        method: String,
        path: String,
        body: String?,
        token: String?,
    ): CoordinatorResult<JSONObject> {
        val url = URL(origin + path)
        val conn = (url.openConnection() as HttpURLConnection).apply {
            requestMethod = method
            connectTimeout = 8_000
            readTimeout = 8_000
            setRequestProperty("Accept", "application/json")
            if (token != null) {
                setRequestProperty("Authorization", "Bearer $token")
            }
            if (body != null) {
                doOutput = true
                setRequestProperty("Content-Type", "application/json")
            }
        }
        try {
            if (body != null) {
                conn.outputStream.use { it.write(body.toByteArray(StandardCharsets.UTF_8)) }
            }
            val stream = if (conn.responseCode in 200..299) conn.inputStream else conn.errorStream
            val text = stream?.bufferedReader(StandardCharsets.UTF_8)?.use { it.readText() }.orEmpty()
            val json = if (text.isBlank()) JSONObject() else JSONObject(text)
            if (conn.responseCode in 200..299) {
                return CoordinatorResult.Ok(json)
            }
            return CoordinatorResult.Err(
                json.optString("error", "http_${conn.responseCode}"),
                json.optString("message", "request failed"),
            )
        } catch (io: IOException) {
            return CoordinatorResult.Err("network", io.message ?: "network error")
        } finally {
            conn.disconnect()
        }
    }

    private fun parseSession(obj: JSONObject): RemoteSession {
        return RemoteSession(
            id = obj.optString("id"),
            state = obj.optString("state"),
            operatorDisplayName = obj.optString("operator_display_name"),
            requestedCapabilities = stringList(obj.optJSONArray("requested_capabilities")),
            grantedCapabilities = stringList(obj.optJSONArray("granted_capabilities")),
            deviceCapabilities = stringList(obj.optJSONArray("device_capabilities")),
            effectiveCapabilities = stringList(obj.optJSONArray("effective_capabilities")),
            sas = obj.optString("sas"),
        )
    }

    private fun stringList(array: JSONArray?): List<String> {
        if (array == null) {
            return emptyList()
        }
        return buildList {
            for (i in 0 until array.length()) {
                add(array.optString(i))
            }
        }
    }

    private fun enc(value: String): String = java.net.URLEncoder.encode(value, "UTF-8")
}
