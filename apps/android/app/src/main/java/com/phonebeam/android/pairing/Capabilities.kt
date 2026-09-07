package com.phonebeam.android.pairing

object Capabilities {
    const val SCREEN_READ = "screen.read"
    const val INPUT_CONTROL = "input.control"
    const val AUDIO_READ = "audio.read"

    val known: List<String> = listOf(SCREEN_READ, INPUT_CONTROL, AUDIO_READ)

    fun isKnown(name: String): Boolean = name in known

    /**
     * Platform-supported capabilities for this build.
     * `input.control` is device-available because AccessibilityService exists;
     * live execution still requires the user to enable it and the PEP to allow it.
     * Audio remains unimplemented.
     */
    fun deviceAvailable(): List<String> = listOf(SCREEN_READ, INPUT_CONTROL)

    fun intersect(requested: List<String>, granted: List<String>, device: List<String>): List<String> {
        return known.filter { it in requested && it in granted && it in device }
    }

    fun subset(inner: List<String>, outer: List<String>): Boolean = inner.all { it in outer }
}
