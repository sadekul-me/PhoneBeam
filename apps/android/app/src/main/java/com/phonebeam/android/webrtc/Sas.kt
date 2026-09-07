package com.phonebeam.android.webrtc

import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

object Sas {
    const val DOMAIN_V1 = "phonebeam-sas-v1"
    const val DOMAIN_V2 = "phonebeam-sas-v2"

    fun normalizeFingerprint(value: String): String {
        return value.lowercase().filter { it in '0'..'9' || it in 'a'..'f' }
    }

    fun validSha256Fingerprint(value: String): Boolean = normalizeFingerprint(value).length == 64

    fun pairingDisplay(
        key: ByteArray,
        origin: String,
        sid: String,
        pid: String,
        operatorId: String,
        requestedCaps: List<String>,
    ): String = digits(hmac(key, DOMAIN_V1, origin, sid, pid, operatorId, requestedCaps, null, null))

    fun mediaDisplay(
        key: ByteArray,
        origin: String,
        sid: String,
        pid: String,
        operatorId: String,
        requestedCaps: List<String>,
        androidFp: String,
        browserFp: String,
    ): String = digits(
        hmac(key, DOMAIN_V2, origin, sid, pid, operatorId, requestedCaps, androidFp, browserFp),
    )

    private fun hmac(
        key: ByteArray,
        domain: String,
        origin: String,
        sid: String,
        pid: String,
        operatorId: String,
        requestedCaps: List<String>,
        androidFp: String?,
        browserFp: String?,
    ): ByteArray {
        val mac = Mac.getInstance("HmacSHA256")
        mac.init(SecretKeySpec(key, "HmacSHA256"))
        mac.update("$domain\n".toByteArray())
        mac.update("origin=$origin\n".toByteArray())
        mac.update("sid=$sid\n".toByteArray())
        mac.update("pid=$pid\n".toByteArray())
        mac.update("operator=$operatorId\n".toByteArray())
        mac.update("caps=${requestedCaps.sorted().joinToString(",")}\n".toByteArray())
        if (androidFp != null && browserFp != null) {
            mac.update("android_fp=${normalizeFingerprint(androidFp)}\n".toByteArray())
            mac.update("browser_fp=${normalizeFingerprint(browserFp)}\n".toByteArray())
        }
        return mac.doFinal()
    }

    private fun digits(sum: ByteArray): String {
        val n = ((sum[0].toInt() and 0xff) shl 24 or
            ((sum[1].toInt() and 0xff) shl 16) or
            ((sum[2].toInt() and 0xff) shl 8) or
            (sum[3].toInt() and 0xff)).toLong() and 0xffff_ffffL
        val six = (n % 1_000_000L).toInt()
        return "%03d %03d".format(six / 1000, six % 1000)
    }
}
