package com.phonebeam.android.webrtc

import android.content.Context
import android.content.Intent
import android.media.projection.MediaProjection
import android.os.Build
import android.util.DisplayMetrics
import android.view.WindowManager
import com.phonebeam.android.PhoneBeamApp
import com.phonebeam.android.control.ControlSession
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONArray
import org.json.JSONObject
import org.webrtc.DefaultVideoDecoderFactory
import org.webrtc.DefaultVideoEncoderFactory
import org.webrtc.EglBase
import org.webrtc.IceCandidate
import org.webrtc.MediaConstraints
import org.webrtc.MediaStreamTrack
import org.webrtc.PeerConnection
import org.webrtc.PeerConnectionFactory
import org.webrtc.ScreenCapturerAndroid
import org.webrtc.SdpObserver
import org.webrtc.SessionDescription
import org.webrtc.SurfaceTextureHelper
import org.webrtc.VideoSource
import org.webrtc.VideoTrack
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.regex.Pattern

class ViewingRuntime(
    private val context: Context,
    private val auth: ViewingAuth,
    private val projectionData: Intent,
    private val listener: Listener,
) {
    interface Listener {
        fun onState(state: String)
        fun onMediaSas(code: String)
        fun onPath(path: String)
        fun onFatal(reason: String)
    }

    private val http = OkHttpClient.Builder().callTimeout(10, TimeUnit.SECONDS).build()
    private var ws: WebSocket? = null
    private var factory: PeerConnectionFactory? = null
    private var egl: EglBase? = null
    private var capturer: ScreenCapturerAndroid? = null
    private var videoSource: VideoSource? = null
    private var videoTrack: VideoTrack? = null
    private var pc: PeerConnection? = null
    private var helper: SurfaceTextureHelper? = null
    private var localFp = ""
    private var remoteFp = ""
    private var iceCompleteSent = false
    private var readySent = false
    private var iceUp = false
    private var iceRestartUsed = false
    private var iceFailGeneration = 0
    private var stopped = false
    private val helloOk = CountDownLatch(1)
    private var control: ControlSession? = null

    fun start() {
        Thread {
            try {
                startLocked()
            } catch (error: Exception) {
                listener.onFatal(error.message ?: "viewing_start_failed")
            }
        }.start()
    }

    private fun startLocked() {
        val ice = fetchIce()
        ensureFactory()
        val servers = ice.servers.map { spec ->
            val builder = PeerConnection.IceServer.builder(spec.urls)
            if (!spec.username.isNullOrBlank() && !spec.credential.isNullOrBlank()) {
                builder.setUsername(spec.username).setPassword(spec.credential)
            }
            builder.createIceServer()
        }
        val rtc = PeerConnection.RTCConfiguration(servers).apply {
            sdpSemantics = PeerConnection.SdpSemantics.UNIFIED_PLAN
            continualGatheringPolicy = PeerConnection.ContinualGatheringPolicy.GATHER_CONTINUALLY
            iceTransportsType = if (ice.transportPolicy == "relay") {
                PeerConnection.IceTransportsType.RELAY
            } else {
                PeerConnection.IceTransportsType.ALL
            }
        }
        val projectionCallback = object : MediaProjection.Callback() {
            override fun onStop() {
                listener.onFatal("projection_revoked")
                stop()
            }
        }
        capturer = ScreenCapturerAndroid(projectionData, projectionCallback)
        videoSource = factory!!.createVideoSource(true)
        helper = SurfaceTextureHelper.create("phonebeam-screen", egl!!.eglBaseContext)
        capturer!!.initialize(helper, context, videoSource!!.capturerObserver)
        val (w, h) = displaySize()
        capturer!!.startCapture(w, h, 24)
        videoTrack = factory!!.createVideoTrack("phonebeam-screen", videoSource)
        pc = factory!!.createPeerConnection(rtc, pcObserver) ?: error("peerconnection")
        val sender = pc!!.addTrack(videoTrack)
        preferVp8(sender?.track())
        val controlSession = ControlSession(
            context,
            auth.sessionId,
            auth.effectiveCaps,
        ) {
            listener.onFatal("session_close")
        }
        control = controlSession
        (context.applicationContext as? PhoneBeamApp)?.controlSession = controlSession
        controlSession.attach(pc!!, videoTrack!!, w, h)
        postProjection("active", w, h)
        connectWs()
        createOffer(iceRestart = false)
    }

    private fun preferVp8(track: MediaStreamTrack?) {
        val peer = pc ?: return
        val transceiver = peer.transceivers.firstOrNull { it.sender.track()?.id() == track?.id() } ?: return
        val caps = factory?.getRtpSenderCapabilities(MediaStreamTrack.MediaType.MEDIA_TYPE_VIDEO) ?: return
        val vp8 = caps.codecs.filter { it.name.equals("VP8", ignoreCase = true) }
        val rest = caps.codecs.filterNot { it.name.equals("VP8", ignoreCase = true) }
        if (vp8.isNotEmpty()) {
            transceiver.setCodecPreferences(vp8 + rest)
        }
    }

    private fun connectWs() {
        val url = auth.origin.replace("https://", "wss://").replace("http://", "ws://") +
            "/api/v1/sessions/${auth.sessionId}/signal"
        val request = Request.Builder().url(url).build()
        val opened = Object()
        ws = http.newWebSocket(
            request,
            object : WebSocketListener() {
                override fun onOpen(webSocket: WebSocket, response: Response) {
                    webSocket.send(JSONObject().put("v", 1).put("type", "hello").put("token", auth.phoneToken).toString())
                    synchronized(opened) { opened.notifyAll() }
                }

                override fun onMessage(webSocket: WebSocket, text: String) {
                    val obj = JSONObject(text)
                    if (obj.optString("type") == "hello_ok") {
                        helloOk.countDown()
                    }
                    handleMessage(text)
                }

                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                    if (!stopped) listener.onFatal("signaling_failed")
                }
            },
        )
        synchronized(opened) { opened.wait(8_000) }
        if (!helloOk.await(8, TimeUnit.SECONDS)) {
            error("hello_timeout")
        }
    }

    private fun createOffer(iceRestart: Boolean) {
        val constraints = MediaConstraints()
        constraints.mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveVideo", "false"))
        constraints.mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveAudio", "false"))
        if (iceRestart) {
            constraints.mandatory.add(MediaConstraints.KeyValuePair("IceRestart", "true"))
            readySent = false
            iceCompleteSent = false
        }
        pc!!.createOffer(
            object : SdpAdapter() {
                override fun onCreateSuccess(sdp: SessionDescription?) {
                    if (sdp == null) return
                    localFp = extractFingerprint(sdp.description)
                    pc!!.setLocalDescription(
                        object : SdpAdapter() {
                            override fun onSetSuccess() {
                                sendJson(
                                    JSONObject()
                                        .put("v", 1)
                                        .put("type", "sdp_offer")
                                        .put("sdp", JSONObject().put("type", "offer").put("sdp", sdp.description)),
                                )
                                if (localFp.isNotEmpty()) {
                                    sendJson(
                                        JSONObject()
                                            .put("v", 1)
                                            .put("type", "peer_fingerprint")
                                            .put(
                                                "fingerprint",
                                                JSONObject()
                                                    .put("algorithm", "sha-256")
                                                    .put("value", localFp)
                                                    .put("role", "phone"),
                                            ),
                                    )
                                }
                            }
                        },
                        sdp,
                    )
                }
            },
            constraints,
        )
    }

    private fun handleMessage(text: String) {
        val obj = JSONObject(text)
        when (obj.optString("type")) {
            "state" -> {
                listener.onState(obj.optString("state"))
                control?.onCoordinatorState(obj.optString("state"))
            }
            "sdp_answer" -> {
                val sdp = obj.getJSONObject("sdp")
                val desc = SessionDescription(SessionDescription.Type.ANSWER, sdp.getString("sdp"))
                remoteFp = extractFingerprint(desc.description)
                pc?.setRemoteDescription(SdpAdapter(), desc)
                maybeReady()
            }
            "ice_candidate" -> {
                val c = obj.getJSONObject("candidate")
                pc?.addIceCandidate(
                    IceCandidate(c.optString("sdp_mid"), c.optInt("sdp_mline_index"), c.getString("candidate")),
                )
            }
            "need_offer" -> {
                iceRestartUsed = true
                iceUp = false
                iceFailGeneration += 1
                createOffer(iceRestart = true)
            }
            "hangup", "error" -> {
                if (obj.optString("error") != "rate_limited") {
                    listener.onFatal(obj.optString("error", obj.optString("type")))
                    stop()
                }
            }
        }
    }

    private fun maybeReady() {
        if (readySent || !iceUp || localFp.isEmpty() || remoteFp.isEmpty()) return
        val code = Sas.mediaDisplay(
            auth.sasMaterial,
            auth.origin,
            auth.sessionId,
            auth.pairingId,
            auth.operatorName,
            auth.requestedCaps,
            localFp,
            remoteFp,
        )
        listener.onMediaSas(code)
        readySent = true
        sendJson(JSONObject().put("v", 1).put("type", "peer_ready"))
    }

    private val pcObserver = object : PeerConnection.Observer {
        override fun onSignalingChange(state: PeerConnection.SignalingState?) = Unit
        override fun onIceConnectionChange(state: PeerConnection.IceConnectionState?) {
            if (state == PeerConnection.IceConnectionState.CONNECTED ||
                state == PeerConnection.IceConnectionState.COMPLETED
            ) {
                iceUp = true
                iceFailGeneration += 1
                maybeReady()
                reportPath()
            }
            if (state == PeerConnection.IceConnectionState.DISCONNECTED) {
                iceUp = false
                listener.onPath("reconnecting")
            }
            if (state == PeerConnection.IceConnectionState.FAILED) {
                iceUp = false
                sendJson(JSONObject().put("v", 1).put("type", "failed_ice"))
                if (iceRestartUsed) {
                    listener.onPath("disconnected")
                    listener.onFatal("failed_ice")
                    stop()
                    return
                }
                val gen = iceFailGeneration + 1
                iceFailGeneration = gen
                listener.onPath("reconnecting")
                Thread {
                    try {
                        Thread.sleep(20_000)
                    } catch (_: InterruptedException) {
                        return@Thread
                    }
                    if (!stopped && iceFailGeneration == gen && !iceUp) {
                        listener.onFatal("failed_ice")
                        stop()
                    }
                }.start()
            }
        }
        override fun onIceConnectionReceivingChange(receiving: Boolean) = Unit
        override fun onIceGatheringChange(state: PeerConnection.IceGatheringState?) {
            if (state == PeerConnection.IceGatheringState.COMPLETE && !iceCompleteSent) {
                iceCompleteSent = true
                sendJson(JSONObject().put("v", 1).put("type", "ice_complete"))
            }
        }
        override fun onIceCandidate(candidate: IceCandidate?) {
            if (candidate == null) return
            sendJson(
                JSONObject()
                    .put("v", 1)
                    .put("type", "ice_candidate")
                    .put(
                        "candidate",
                        JSONObject()
                            .put("candidate", candidate.sdp)
                            .put("sdp_mid", candidate.sdpMid)
                            .put("sdp_mline_index", candidate.sdpMLineIndex),
                    ),
            )
        }
        override fun onIceCandidatesRemoved(candidates: Array<out IceCandidate>?) = Unit
        override fun onAddStream(stream: org.webrtc.MediaStream?) = Unit
        override fun onRemoveStream(stream: org.webrtc.MediaStream?) = Unit
        override fun onDataChannel(channel: org.webrtc.DataChannel?) = Unit
        override fun onRenegotiationNeeded() = Unit
        override fun onAddTrack(receiver: org.webrtc.RtpReceiver?, streams: Array<out org.webrtc.MediaStream>?) = Unit
    }

    private fun reportPath() {
        pc?.getStats { report ->
            var path = "direct"
            report.statsMap.values.forEach { stats ->
                if (stats.type == "candidate-pair" && stats.members["state"]?.toString() == "succeeded") {
                    val remoteId = stats.members["remoteCandidateId"]?.toString()
                    val remote = remoteId?.let { report.statsMap[it] }
                    val typ = remote?.members?.get("candidateType")?.toString()
                    if (typ == "relay") path = "relay"
                }
            }
            listener.onPath(path)
            sendJson(
                JSONObject().put("v", 1).put("type", "stats").put("stats", JSONObject().put("path", path)),
            )
        }
    }

    private fun sendJson(obj: JSONObject) {
        ws?.send(obj.toString())
    }

    private fun postProjection(status: String, width: Int, height: Int) {
        val url = URL(auth.origin + "/api/v1/sessions/${auth.sessionId}/projection")
        val conn = url.openConnection() as HttpURLConnection
        conn.requestMethod = "POST"
        conn.connectTimeout = 8000
        conn.readTimeout = 8000
        conn.doOutput = true
        conn.setRequestProperty("Authorization", "Bearer ${auth.phoneToken}")
        conn.setRequestProperty("Content-Type", "application/json")
        val body = JSONObject()
            .put("status", status)
            .put("width", width)
            .put("height", height)
            .put("scope", "unknown")
            .toString()
        conn.outputStream.use { it.write(body.toByteArray()) }
        val code = conn.responseCode
        val stream = if (code in 200..299) conn.inputStream else conn.errorStream
        stream?.close()
        conn.disconnect()
        if (code !in 200..299) {
            error("projection_report_failed")
        }
    }

    private fun fetchIce(): IceConfig {
        val url = java.net.URL(auth.origin + "/api/v1/sessions/${auth.sessionId}/ice")
        val conn = url.openConnection() as java.net.HttpURLConnection
        conn.setRequestProperty("Authorization", "Bearer ${auth.phoneToken}")
        conn.connectTimeout = 8000
        conn.readTimeout = 8000
        val text = conn.inputStream.bufferedReader().use { it.readText() }
        val json = JSONObject(text)
        val servers = json.optJSONArray("ice_servers") ?: JSONArray()
        val list = buildList {
            for (i in 0 until servers.length()) {
                val item = servers.getJSONObject(i)
                val urls = item.getJSONArray("urls")
                add(
                    IceServerJson(
                        urls = (0 until urls.length()).map { urls.getString(it) },
                        username = item.optString("username").ifBlank { null },
                        credential = item.optString("credential").ifBlank { null },
                    ),
                )
            }
        }
        return IceConfig(
            servers = list,
            transportPolicy = json.optString("ice_transport_policy", "all"),
            turnConfigured = json.optBoolean("turn_configured"),
        )
    }

    private fun ensureFactory() {
        if (factory != null) return
        PeerConnectionFactory.initialize(
            PeerConnectionFactory.InitializationOptions.builder(context).createInitializationOptions(),
        )
        egl = EglBase.create()
        factory = PeerConnectionFactory.builder()
            .setVideoEncoderFactory(DefaultVideoEncoderFactory(egl!!.eglBaseContext, true, true))
            .setVideoDecoderFactory(DefaultVideoDecoderFactory(egl!!.eglBaseContext))
            .createPeerConnectionFactory()
    }

    @Suppress("DEPRECATION")
    private fun displaySize(): Pair<Int, Int> {
        val wm = context.getSystemService(Context.WINDOW_SERVICE) as WindowManager
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            val bounds = wm.currentWindowMetrics.bounds
            bounds.width() to bounds.height()
        } else {
            val metrics = DisplayMetrics()
            wm.defaultDisplay.getRealMetrics(metrics)
            metrics.widthPixels to metrics.heightPixels
        }
    }

    fun stop() {
        if (stopped) return
        stopped = true
        try {
            sendJson(JSONObject().put("v", 1).put("type", "hangup"))
        } catch (_: Exception) {
        }
        ws?.close(1000, "hangup")
        ws = null
        control?.stop()
        control = null
        (context.applicationContext as? PhoneBeamApp)?.controlSession = null
        try {
            capturer?.stopCapture()
        } catch (_: Exception) {
        }
        capturer?.dispose()
        capturer = null
        videoTrack?.dispose()
        videoTrack = null
        videoSource?.dispose()
        videoSource = null
        helper?.dispose()
        helper = null
        pc?.close()
        pc?.dispose()
        pc = null
        factory?.dispose()
        factory = null
        egl?.release()
        egl = null
    }

    companion object {
        private val fpPattern = Pattern.compile("a=fingerprint:sha-256 ([0-9A-Fa-f: ]+)", Pattern.CASE_INSENSITIVE)

        fun extractFingerprint(sdp: String): String {
            val matcher = fpPattern.matcher(sdp)
            return if (matcher.find()) Sas.normalizeFingerprint(matcher.group(1) ?: "") else ""
        }
    }
}

private open class SdpAdapter : SdpObserver {
    override fun onCreateSuccess(sdp: SessionDescription?) = Unit
    override fun onSetSuccess() = Unit
    override fun onCreateFailure(error: String?) = Unit
    override fun onSetFailure(error: String?) = Unit
}
