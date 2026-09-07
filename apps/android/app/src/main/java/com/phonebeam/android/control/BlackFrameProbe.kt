package com.phonebeam.android.control

import org.webrtc.VideoFrame
import org.webrtc.VideoSink
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Heuristic FLAG_SECURE / black-frame detector from the local capture track.
 * Does not read the accessibility tree.
 */
class BlackFrameProbe : VideoSink {
    private val protectedFlag = AtomicBoolean(false)
    private var blackStreak = 0
    private var seenFrame = false

    fun isProtected(): Boolean = protectedFlag.get()

    fun hasFrame(): Boolean = seenFrame

    override fun onFrame(frame: VideoFrame?) {
        if (frame == null) {
            return
        }
        val buffer = frame.buffer.toI420() ?: return
        seenFrame = true
        try {
            val y = buffer.dataY
            val width = buffer.width
            val height = buffer.height
            val stride = buffer.strideY
            if (width <= 0 || height <= 0) {
                return
            }
            val samples = intArrayOf(
                sample(y, stride, width / 2, height / 2),
                sample(y, stride, width / 4, height / 4),
                sample(y, stride, width * 3 / 4, height / 4),
                sample(y, stride, width / 4, height * 3 / 4),
                sample(y, stride, width * 3 / 4, height * 3 / 4),
            )
            val black = samples.all { it < 18 }
            if (black) {
                blackStreak += 1
                if (blackStreak >= 8) {
                    protectedFlag.set(true)
                }
            } else {
                blackStreak = 0
                protectedFlag.set(false)
            }
        } finally {
            buffer.release()
        }
    }

    private fun sample(y: java.nio.ByteBuffer, stride: Int, x: Int, py: Int): Int {
        val index = py * stride + x
        if (index < 0 || index >= y.limit()) {
            return 255
        }
        return y.get(index).toInt() and 0xff
    }
}
