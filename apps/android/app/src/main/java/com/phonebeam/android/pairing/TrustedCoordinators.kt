package com.phonebeam.android.pairing

/**
 * Development allowlist only. A QR origin that is not in this set is never contacted.
 * Add a LAN origin here when testing on a physical device, e.g. http://192.168.1.10:8080
 */
object TrustedCoordinators {
    val developmentAllowlist: Set<String> = setOf(
        "http://127.0.0.1:8080",
        "http://localhost:8080",
        "http://10.0.2.2:8080",
    )

    fun normalize(origin: String): String = origin.trim().trimEnd('/')

    fun isTrusted(origin: String): Boolean = normalize(origin) in developmentAllowlist
}
