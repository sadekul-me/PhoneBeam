package com.phonebeam.android

import android.app.Application
import com.phonebeam.android.capture.CaptureSession
import com.phonebeam.android.control.ControlSession
import com.phonebeam.android.webrtc.ViewingAuth

class PhoneBeamApp : Application() {
    val captureSession = CaptureSession()
    @Volatile var viewingAuth: ViewingAuth? = null
    @Volatile var viewingState: String = ""
    @Volatile var mediaSas: String = ""
    @Volatile var connectionPath: String = ""
    @Volatile var liveControlCaps: List<String> = emptyList()
    @Volatile var controlSession: ControlSession? = null

    fun notifyAccessibilityChanged(enabled: Boolean) {
        controlSession?.onAccessibilityChanged()
    }

    fun clearViewing() {
        viewingAuth = null
        viewingState = ""
        mediaSas = ""
        connectionPath = ""
        liveControlCaps = emptyList()
        controlSession = null
    }
}
