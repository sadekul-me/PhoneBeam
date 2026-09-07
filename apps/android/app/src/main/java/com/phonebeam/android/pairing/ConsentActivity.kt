package com.phonebeam.android.pairing

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.util.Base64
import android.view.View
import android.widget.CheckBox
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import com.phonebeam.android.PhoneBeamApp
import com.phonebeam.android.R
import com.phonebeam.android.capture.CaptureEffect
import com.phonebeam.android.capture.CaptureEvent
import com.phonebeam.android.capture.CaptureService
import com.phonebeam.android.databinding.ActivityConsentBinding
import com.phonebeam.android.webrtc.ViewingActivity
import com.phonebeam.android.webrtc.ViewingAuth
import com.phonebeam.android.webrtc.ViewingPolicy
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class ConsentActivity : AppCompatActivity() {

    private lateinit var binding: ActivityConsentBinding
    private lateinit var api: CoordinatorApi
    private var pendingToken: String? = null
    private var phoneToken: String? = null
    private var sessionId: String = ""
    private var pairingId: String = ""
    private var origin: String = ""
    private val grantBoxes = mutableListOf<CheckBox>()
    private var approvedSession: RemoteSession? = null

    private val captureSession get() = (application as PhoneBeamApp).captureSession

    private val notificationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) launchMediaProjectionConsent()
    }

    private val mediaProjectionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult(),
    ) { result ->
        val data = result.data
        val token = phoneToken
        val sess = approvedSession
        if (result.resultCode == RESULT_OK && data != null && token != null && sess != null) {
            val material = decodeMaterial(sess.sasMaterial)
            (application as PhoneBeamApp).viewingAuth = ViewingAuth(
                origin = origin,
                sessionId = sessionId,
                pairingId = pairingId,
                phoneToken = token,
                sasMaterial = material,
                operatorName = sess.operatorDisplayName,
                pairingSas = sess.sas,
                requestedCaps = sess.requestedCapabilities,
                effectiveCaps = sess.effectiveCapabilities,
            )
            val transition = captureSession.dispatch(CaptureEvent.PermissionGranted)
            if (transition.effect == CaptureEffect.StartCaptureService) {
                CaptureService.start(this, result.resultCode, data)
                startActivity(Intent(this, ViewingActivity::class.java))
            }
        } else {
            captureSession.dispatch(CaptureEvent.PermissionDenied)
            lifecycleScope.launch {
                withContext(Dispatchers.IO) {
                    phoneToken?.let { api.reportProjection(sessionId, it, "denied") }
                }
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityConsentBinding.inflate(layoutInflater)
        setContentView(binding.root)
        binding.toolbar.setNavigationOnClickListener { finish() }

        origin = intent.getStringExtra(EXTRA_ORIGIN).orEmpty()
        sessionId = intent.getStringExtra(EXTRA_SID).orEmpty()
        pairingId = intent.getStringExtra(EXTRA_PID).orEmpty()
        if (!TrustedCoordinators.isTrusted(origin) || sessionId.isBlank() || pairingId.isBlank()) {
            finish()
            return
        }
        api = CoordinatorApi(origin)
        binding.approveButton.setOnClickListener { submitApproval() }
        binding.rejectButton.setOnClickListener { submitRejection() }
        binding.startShareButton.setOnClickListener { startRemoteShare() }
        loadScan()
    }

    private fun loadScan() {
        setBusy(true)
        lifecycleScope.launch {
            val result = withContext(Dispatchers.IO) { api.scan(sessionId, pairingId) }
            setBusy(false)
            when (result) {
                is CoordinatorResult.Err -> showTerminal(result.code, result.message)
                is CoordinatorResult.Ok -> renderRequest(result.value.session, result.value.phonePendingToken)
            }
        }
    }

    private fun renderRequest(session: RemoteSession, token: String) {
        pendingToken = token
        binding.requestCard.visibility = View.VISIBLE
        binding.resultCard.visibility = View.GONE
        binding.operatorValue.text = session.operatorDisplayName
        binding.sessionValue.text = session.id
        binding.stateValue.text = session.state
        binding.sasValue.text = session.sas
        binding.capabilityList.removeAllViews()
        grantBoxes.clear()
        session.requestedCapabilities.forEach { cap ->
            val box = CheckBox(this).apply {
                text = cap
                isChecked = cap in Capabilities.deviceAvailable()
            }
            grantBoxes.add(box)
            binding.capabilityList.addView(box)
        }
        binding.deviceValue.text = Capabilities.deviceAvailable().joinToString(", ")
        binding.approveButton.isEnabled = true
        binding.rejectButton.isEnabled = true
    }

    private fun submitApproval() {
        val token = pendingToken ?: return
        val granted = grantBoxes.filter { it.isChecked }.map { it.text.toString() }
        setBusy(true)
        lifecycleScope.launch {
            val result = withContext(Dispatchers.IO) {
                api.approve(sessionId, token, granted, Capabilities.deviceAvailable())
            }
            setBusy(false)
            when (result) {
                is CoordinatorResult.Err -> showTerminal(result.code, result.message)
                is CoordinatorResult.Ok -> {
                    phoneToken = result.value.phoneToken
                    showResult(result.value.session)
                }
            }
        }
    }

    private fun submitRejection() {
        val token = pendingToken ?: return
        setBusy(true)
        lifecycleScope.launch {
            val result = withContext(Dispatchers.IO) { api.reject(sessionId, token) }
            setBusy(false)
            when (result) {
                is CoordinatorResult.Err -> showTerminal(result.code, result.message)
                is CoordinatorResult.Ok -> showResult(result.value)
            }
        }
    }

    private fun showResult(session: RemoteSession) {
        pendingToken = null
        approvedSession = session
        binding.requestCard.visibility = View.VISIBLE
        binding.resultCard.visibility = View.VISIBLE
        binding.approveButton.isEnabled = false
        binding.rejectButton.isEnabled = false
        grantBoxes.forEach { it.isEnabled = false }
        binding.stateValue.text = session.state
        binding.resultState.text = session.state
        binding.effectiveValue.text = session.effectiveCapabilities.joinToString(", ").ifBlank {
            getString(R.string.value_unavailable)
        }
        binding.resultDetail.text = getString(R.string.pairing_no_stream)
        val canShare = ViewingPolicy.canStartRemoteShare(
            session.state,
            session.effectiveCapabilities,
        )
        binding.startShareButton.visibility = if (canShare) View.VISIBLE else View.GONE
        val wantsControl = "input.control" in session.effectiveCapabilities
        binding.accessibilitySettingsButton.visibility = if (wantsControl) View.VISIBLE else View.GONE
        binding.accessibilitySettingsButton.setOnClickListener {
            startActivity(Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS))
        }
    }

    private fun startRemoteShare() {
        val transition = captureSession.dispatch(CaptureEvent.StartRequested)
        if (transition.effect == CaptureEffect.RequestSystemPermission) {
            requestCapturePermissions()
        }
        phoneToken?.let { token ->
            lifecycleScope.launch {
                withContext(Dispatchers.IO) { api.reportProjection(sessionId, token, "pending") }
            }
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

    private fun showTerminal(code: String, message: String) {
        pendingToken = null
        binding.requestCard.visibility = View.GONE
        binding.resultCard.visibility = View.VISIBLE
        binding.approveButton.isEnabled = false
        binding.rejectButton.isEnabled = false
        binding.startShareButton.visibility = View.GONE
        binding.resultState.text = code
        binding.effectiveValue.text = getString(R.string.value_unavailable)
        binding.resultDetail.text = message
        binding.stateValue.text = code
    }

    private fun setBusy(busy: Boolean) {
        binding.progress.visibility = if (busy) View.VISIBLE else View.GONE
        binding.approveButton.isEnabled = !busy && pendingToken != null
        binding.rejectButton.isEnabled = !busy && pendingToken != null
    }

    private fun decodeMaterial(raw: String): ByteArray {
        return Base64.decode(raw, Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING)
    }

    companion object {
        private const val EXTRA_ORIGIN = "origin"
        private const val EXTRA_SID = "sid"
        private const val EXTRA_PID = "pid"

        fun intent(context: Context, payload: PairingQr): Intent {
            return Intent(context, ConsentActivity::class.java)
                .putExtra(EXTRA_ORIGIN, payload.origin)
                .putExtra(EXTRA_SID, payload.sessionId)
                .putExtra(EXTRA_PID, payload.pairingId)
        }
    }
}
