package com.phonebeam.android

import android.app.Application
import com.phonebeam.android.capture.CaptureSession

class PhoneBeamApp : Application() {
    val captureSession = CaptureSession()
}
