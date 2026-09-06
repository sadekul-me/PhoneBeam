package com.phonebeam.android

import android.Manifest
import android.content.pm.PackageManager
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.Bundle
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.core.view.isVisible
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import com.phonebeam.android.capture.CaptureEffect
import com.phonebeam.android.capture.CaptureEvent
import com.phonebeam.android.capture.CaptureService
import com.phonebeam.android.capture.CaptureState
import com.phonebeam.android.capture.CaptureUiState
import com.phonebeam.android.databinding.ActivityMainBinding
import com.phonebeam.android.pairing.ScanActivity
import kotlinx.coroutines.launch

class MainActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMainBinding
    private val session by lazy { (application as PhoneBeamApp).captureSession }
    private var lastShownPreview: android.graphics.Bitmap? = null

    private val notificationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) {
            launchMediaProjectionConsent()
        } else {
            session.dispatch(
                CaptureEvent.Failed(getString(R.string.error_notification_required)),
            )
        }
    }

    private val mediaProjectionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        val data = result.data
        if (result.resultCode == RESULT_OK && data != null) {
            val transition = session.dispatch(CaptureEvent.PermissionGranted)
            if (transition.effect == CaptureEffect.StartCaptureService) {
                try {
                    CaptureService.start(this, result.resultCode, data)
                } catch (error: RuntimeException) {
                    session.dispatch(
                        CaptureEvent.Failed(error.message ?: "service_start_failed"),
                    )
                }
            }
        } else {
            session.dispatch(CaptureEvent.PermissionDenied)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        binding.startButton.setOnClickListener { onStartClicked() }
        binding.stopButton.setOnClickListener { onStopClicked() }
        binding.scanPairingButton.setOnClickListener {
            startActivity(android.content.Intent(this, ScanActivity::class.java))
        }

        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.STARTED) {
                session.ui.collect { render(it) }
            }
        }
    }

    private fun onStartClicked() {
        val transition = session.dispatch(CaptureEvent.StartRequested)
        if (transition.effect == CaptureEffect.RequestSystemPermission) {
            requestCapturePermissions()
        }
    }

    private fun onStopClicked() {
        val transition = session.dispatch(CaptureEvent.StopRequested)
        if (transition.effect == CaptureEffect.StopCapture) {
            CaptureService.stop(this)
        }
    }

    private fun requestCapturePermissions() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) !=
            PackageManager.PERMISSION_GRANTED
        ) {
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        } else {
            launchMediaProjectionConsent()
        }
    }

    private fun launchMediaProjectionConsent() {
        val manager = getSystemService(MediaProjectionManager::class.java)
        mediaProjectionLauncher.launch(manager.createScreenCaptureIntent())
    }

    private fun render(ui: CaptureUiState) {
        val stateLabel = when (ui.state) {
            CaptureState.IDLE -> getString(R.string.state_idle)
            CaptureState.REQUESTING_PERMISSION -> getString(R.string.state_requesting)
            CaptureState.STARTING -> getString(R.string.state_starting)
            CaptureState.ACTIVE -> getString(R.string.state_active)
            CaptureState.STOPPING -> getString(R.string.state_stopping)
            CaptureState.STOPPED -> getString(R.string.state_stopped)
            CaptureState.ERROR -> getString(R.string.state_error)
        }
        binding.stateValue.text = stateLabel

        val sharing = ui.state == CaptureState.ACTIVE ||
            ui.state == CaptureState.STARTING ||
            ui.state == CaptureState.STOPPING
        binding.startButton.isVisible = !sharing
        binding.stopButton.isVisible = sharing
        binding.startButton.isEnabled =
            ui.state == CaptureState.IDLE ||
                ui.state == CaptureState.STOPPED ||
                ui.state == CaptureState.ERROR
        binding.stopButton.isEnabled = ui.state == CaptureState.ACTIVE || ui.state == CaptureState.STARTING

        binding.statsCard.isVisible = ui.state == CaptureState.ACTIVE || ui.stats.framesReceived > 0
        binding.framesValue.text = getString(R.string.frames_format, ui.stats.framesReceived)
        binding.sizeValue.text = if (ui.stats.width > 0) {
            getString(R.string.size_format, ui.stats.width, ui.stats.height, ui.stats.densityDpi)
        } else {
            getString(R.string.value_unavailable)
        }
        binding.fpsValue.text = getString(R.string.fps_format, ui.stats.framesPerSecond)

        val preview = ui.preview
        binding.previewImage.isVisible = preview != null
        binding.previewCaption.isVisible = preview != null
        if (preview != lastShownPreview) {
            binding.previewImage.setImageBitmap(preview)
            lastShownPreview = preview
        }

        val detail = when {
            ui.state == CaptureState.ERROR -> ui.errorReason
            ui.stopReason == "permission_denied" && ui.state == CaptureState.STOPPED ->
                getString(R.string.detail_permission_denied)
            ui.stopReason == "projection_revoked" && ui.state == CaptureState.STOPPED ->
                getString(R.string.detail_projection_revoked)
            else -> null
        }
        binding.detailText.isVisible = !detail.isNullOrBlank()
        binding.detailText.text = detail
    }
}
