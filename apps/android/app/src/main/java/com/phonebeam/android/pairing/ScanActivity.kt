package com.phonebeam.android.pairing

import android.Manifest
import android.content.pm.PackageManager
import android.os.Bundle
import android.util.Size
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ExperimentalGetImage
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.Preview
import androidx.camera.core.resolutionselector.ResolutionSelector
import androidx.camera.core.resolutionselector.ResolutionStrategy
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.core.content.ContextCompat
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import com.phonebeam.android.R
import com.phonebeam.android.databinding.ActivityScanBinding
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

class ScanActivity : AppCompatActivity() {

    private lateinit var binding: ActivityScanBinding
    private val cameraExecutor = Executors.newSingleThreadExecutor()
    private val handled = AtomicBoolean(false)
    private val scanner = BarcodeScanning.getClient(
        BarcodeScannerOptions.Builder().setBarcodeFormats(Barcode.FORMAT_QR_CODE).build(),
    )

    private val cameraPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) {
            startCamera()
        } else {
            Toast.makeText(this, R.string.pairing_camera_required, Toast.LENGTH_LONG).show()
            finish()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityScanBinding.inflate(layoutInflater)
        setContentView(binding.root)
        binding.toolbar.setNavigationOnClickListener { finish() }

        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) ==
            PackageManager.PERMISSION_GRANTED
        ) {
            startCamera()
        } else {
            cameraPermission.launch(Manifest.permission.CAMERA)
        }
    }

    @androidx.annotation.OptIn(ExperimentalGetImage::class)
    private fun startCamera() {
        val future = ProcessCameraProvider.getInstance(this)
        future.addListener(
            {
                val provider = future.get()
                val preview = Preview.Builder().build().also {
                    it.surfaceProvider = binding.previewView.surfaceProvider
                }
                val analysis = ImageAnalysis.Builder()
                    .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                    .setResolutionSelector(
                        ResolutionSelector.Builder()
                            .setResolutionStrategy(
                                ResolutionStrategy(Size(1280, 720), ResolutionStrategy.FALLBACK_RULE_CLOSEST_HIGHER_THEN_LOWER),
                            )
                            .build(),
                    )
                    .build()
                analysis.setAnalyzer(cameraExecutor) { proxy ->
                    val media = proxy.image
                    if (media == null || handled.get()) {
                        proxy.close()
                        return@setAnalyzer
                    }
                    val image = InputImage.fromMediaImage(media, proxy.imageInfo.rotationDegrees)
                    scanner.process(image)
                        .addOnSuccessListener { barcodes ->
                            val raw = barcodes.firstOrNull { !it.rawValue.isNullOrBlank() }?.rawValue
                            if (raw != null) {
                                onRawQr(raw)
                            }
                        }
                        .addOnCompleteListener { proxy.close() }
                }
                try {
                    provider.unbindAll()
                    provider.bindToLifecycle(this, CameraSelector.DEFAULT_BACK_CAMERA, preview, analysis)
                } catch (_: RuntimeException) {
                    Toast.makeText(this, R.string.pairing_camera_required, Toast.LENGTH_LONG).show()
                    finish()
                }
            },
            ContextCompat.getMainExecutor(this),
        )
    }

    private fun onRawQr(raw: String) {
        if (!handled.compareAndSet(false, true)) {
            return
        }
        val parsed = PairingQrParser.parse(raw, nowEpochSeconds = System.currentTimeMillis() / 1000L)
        runOnUiThread {
            when (parsed) {
                is PairingQrParse.Rejected -> {
                    val message = when (parsed.reason) {
                        PairingQrReject.MALFORMED -> getString(R.string.pairing_reject_malformed)
                        PairingQrReject.UNKNOWN_VERSION -> getString(R.string.pairing_reject_version)
                        PairingQrReject.UNTRUSTED_ORIGIN -> getString(R.string.pairing_reject_origin)
                        PairingQrReject.EXPIRED -> getString(R.string.pairing_reject_expired)
                    }
                    Toast.makeText(this, message, Toast.LENGTH_LONG).show()
                    handled.set(false)
                }
                is PairingQrParse.Ok -> {
                    startActivity(ConsentActivity.intent(this, parsed.payload))
                    finish()
                }
            }
        }
    }

    override fun onDestroy() {
        super.onDestroy()
        cameraExecutor.shutdown()
        scanner.close()
    }
}
