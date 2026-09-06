package com.phonebeam.android.pairing

import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.view.View
import android.widget.CheckBox
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import com.phonebeam.android.R
import com.phonebeam.android.databinding.ActivityConsentBinding
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class ConsentActivity : AppCompatActivity() {

    private lateinit var binding: ActivityConsentBinding
    private lateinit var api: CoordinatorApi
    private var pendingToken: String? = null
    private var sessionId: String = ""
    private val grantBoxes = mutableListOf<CheckBox>()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityConsentBinding.inflate(layoutInflater)
        setContentView(binding.root)
        binding.toolbar.setNavigationOnClickListener { finish() }

        val origin = intent.getStringExtra(EXTRA_ORIGIN).orEmpty()
        sessionId = intent.getStringExtra(EXTRA_SID).orEmpty()
        val pairingId = intent.getStringExtra(EXTRA_PID).orEmpty()
        if (!TrustedCoordinators.isTrusted(origin) || sessionId.isBlank() || pairingId.isBlank()) {
            finish()
            return
        }
        api = CoordinatorApi(origin)
        binding.approveButton.setOnClickListener { submitApproval() }
        binding.rejectButton.setOnClickListener { submitRejection() }
        loadScan(pairingId)
    }

    private fun loadScan(pairingId: String) {
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
                is CoordinatorResult.Ok -> showResult(result.value.session)
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
    }

    private fun showTerminal(code: String, message: String) {
        pendingToken = null
        binding.requestCard.visibility = View.GONE
        binding.resultCard.visibility = View.VISIBLE
        binding.approveButton.isEnabled = false
        binding.rejectButton.isEnabled = false
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
