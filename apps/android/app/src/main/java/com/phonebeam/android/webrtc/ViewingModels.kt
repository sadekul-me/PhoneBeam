package com.phonebeam.android.webrtc

data class ViewingAuth(
    val origin: String,
    val sessionId: String,
    val pairingId: String,
    val phoneToken: String,
    val sasMaterial: ByteArray,
    val operatorName: String,
    val pairingSas: String,
    val requestedCaps: List<String>,
    val effectiveCaps: List<String>,
)

data class IceServerJson(
    val urls: List<String>,
    val username: String?,
    val credential: String?,
)

data class IceConfig(
    val servers: List<IceServerJson>,
    val transportPolicy: String,
    val turnConfigured: Boolean,
)
