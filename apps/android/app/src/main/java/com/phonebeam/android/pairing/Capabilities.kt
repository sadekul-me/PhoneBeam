package com.phonebeam.android.pairing

object Capabilities {
    const val SCREEN_READ = "screen.read"
    const val INPUT_CONTROL = "input.control"
    const val AUDIO_READ = "audio.read"

    val known: List<String> = listOf(SCREEN_READ, INPUT_CONTROL, AUDIO_READ)

    fun isKnown(name: String): Boolean = name in known

    /**
     * M1 reports what this build can actually enforce later.
     * MediaProjection exists (M0) so screen.read is available.
     * AccessibilityService and audio capture are not in this milestone.
     */
    fun deviceAvailable(): List<String> = listOf(SCREEN_READ)

    fun intersect(requested: List<String>, granted: List<String>, device: List<String>): List<String> {
        return known.filter { it in requested && it in granted && it in device }
    }

    fun subset(inner: List<String>, outer: List<String>): Boolean = inner.all { it in outer }
}
