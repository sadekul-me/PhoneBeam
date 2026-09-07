package com.phonebeam.android.webrtc

import android.content.Intent
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import androidx.appcompat.app.AppCompatActivity
import com.phonebeam.android.PhoneBeamApp
import com.phonebeam.android.R
import com.phonebeam.android.capture.CaptureEffect
import com.phonebeam.android.capture.CaptureEvent
import com.phonebeam.android.capture.CaptureService
import com.phonebeam.android.databinding.ActivityViewingBinding

class ViewingActivity : AppCompatActivity() {
    private lateinit var binding: ActivityViewingBinding
    private val handler = Handler(Looper.getMainLooper())
    private val poll = object : Runnable {
        override fun run() {
            val app = application as PhoneBeamApp
            binding.stateValue.text = app.viewingState.ifBlank { app.captureSession.currentState().name }
            binding.mediaSas.text = app.mediaSas.ifBlank { "—" }
            binding.pathValue.text = app.connectionPath.ifBlank { "connecting" }
            val live = app.liveControlCaps
            binding.controlValue.text = if ("input.control" in live) {
                getString(R.string.viewing_control_on)
            } else {
                getString(R.string.viewing_control_off)
            }
            handler.postDelayed(this, 500)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityViewingBinding.inflate(layoutInflater)
        setContentView(binding.root)
        val auth = (application as PhoneBeamApp).viewingAuth
        binding.operatorValue.text = auth?.operatorName ?: ""
        binding.pairingSas.text = auth?.pairingSas ?: ""
        binding.accessibilitySettingsButton.setOnClickListener {
            startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS))
        }
        binding.disconnectButton.setOnClickListener {
            val transition = (application as PhoneBeamApp).captureSession.dispatch(CaptureEvent.StopRequested)
            if (transition.effect == CaptureEffect.StopCapture) {
                CaptureService.stop(this)
            }
            finish()
        }
    }

    override fun onStart() {
        super.onStart()
        handler.post(poll)
    }

    override fun onStop() {
        handler.removeCallbacks(poll)
        super.onStop()
    }
}
