package com.phonebeam.android.control

data class DisplayPoint(val x: Int, val y: Int)

object Geometry {
    fun mapNormalized(x: Double, y: Double, frameWidth: Int, frameHeight: Int): DisplayPoint? {
        if (frameWidth <= 0 || frameHeight <= 0) {
            return null
        }
        if (x.isNaN() || y.isNaN() || x < 0.0 || x > 1.0 || y < 0.0 || y > 1.0) {
            return null
        }
        val px = (x * (frameWidth - 1).coerceAtLeast(0)).toInt().coerceIn(0, frameWidth - 1)
        val py = (y * (frameHeight - 1).coerceAtLeast(0)).toInt().coerceIn(0, frameHeight - 1)
        return DisplayPoint(px, py)
    }
}
