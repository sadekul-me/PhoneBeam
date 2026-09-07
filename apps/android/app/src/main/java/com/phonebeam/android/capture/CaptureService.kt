package com.phonebeam.android.capture

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.graphics.Bitmap
import android.graphics.PixelFormat
import android.hardware.display.DisplayManager
import android.hardware.display.VirtualDisplay
import android.media.Image
import android.media.ImageReader
import android.media.projection.MediaProjection
import android.media.projection.MediaProjectionManager
import android.os.Build
import android.os.Handler
import android.os.HandlerThread
import android.os.IBinder
import android.os.SystemClock
import android.util.DisplayMetrics
import android.view.Display
import androidx.core.app.NotificationCompat
import com.phonebeam.android.MainActivity
import com.phonebeam.android.PhoneBeamApp
import com.phonebeam.android.R
import com.phonebeam.android.webrtc.ViewingActivity
import com.phonebeam.android.webrtc.ViewingRuntime

/**
 * Visible MediaProjection capture. START_NOT_STICKY: process death does not silently recapture.
 */
class CaptureService : Service() {

    private val session: CaptureSession
        get() = (application as PhoneBeamApp).captureSession

    private var mediaProjection: MediaProjection? = null
    private var virtualDisplay: VirtualDisplay? = null
    private var imageReader: ImageReader? = null
    private var captureThread: HandlerThread? = null
    private var captureHandler: Handler? = null
    private var displayManager: DisplayManager? = null

    private var framesReceived: Long = 0
    private var windowStartMs: Long = 0
    private var windowFrames: Long = 0
    private var currentFps: Float = 0f
    private var lastPreviewFrame: Long = 0
    private var capturing: Boolean = false
    private var viewing: ViewingRuntime? = null
    private var remoteMode: Boolean = false

    private val projectionCallback = object : MediaProjection.Callback() {
        override fun onStop() {
            Handler(mainLooper).post {
                if (!capturing) return@post
                session.dispatch(CaptureEvent.ProjectionRevoked)
                tearDownCapture(notifyStopped = true)
            }
        }

        override fun onCapturedContentResize(width: Int, height: Int) {
            Handler(mainLooper).post {
                if (capturing && width > 0 && height > 0) {
                    resizeCapture(width, height, currentDensityDpi())
                }
            }
        }
    }

    private val displayListener = object : DisplayManager.DisplayListener {
        override fun onDisplayAdded(displayId: Int) = Unit
        override fun onDisplayRemoved(displayId: Int) = Unit
        override fun onDisplayChanged(displayId: Int) {
            if (!capturing || displayId != Display.DEFAULT_DISPLAY) return
            val (width, height, dpi) = currentDisplayMetrics()
            resizeCapture(width, height, dpi)
        }
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        displayManager = getSystemService(DisplayManager::class.java)
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent == null) {
            stopSelf()
            return START_NOT_STICKY
        }
        when (intent.action) {
            ACTION_START -> startCapture(intent)
            ACTION_STOP -> {
                if (!capturing) {
                    stopSelf()
                    return START_NOT_STICKY
                }
                session.dispatch(CaptureEvent.StopRequested)
                tearDownCapture(notifyStopped = true)
            }
            else -> stopSelf()
        }
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        tearDownCapture(notifyStopped = session.currentState() == CaptureState.ACTIVE ||
            session.currentState() == CaptureState.STARTING ||
            session.currentState() == CaptureState.STOPPING)
        super.onDestroy()
    }

    private fun startCapture(intent: Intent) {
        if (capturing) {
            return
        }
        val resultCode = intent.getIntExtra(EXTRA_RESULT_CODE, 0)
        val resultData = projectionResultIntent(intent)
        if (resultCode != android.app.Activity.RESULT_OK || resultData == null) {
            session.dispatch(CaptureEvent.Failed("missing_projection_result"))
            stopSelf()
            return
        }

        remoteMode = (application as PhoneBeamApp).viewingAuth != null
        val notification = buildNotification()
        startForeground(
            NOTIFICATION_ID,
            notification,
            ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PROJECTION,
        )

        val app = application as PhoneBeamApp
        val auth = app.viewingAuth
        if (auth != null) {
            remoteMode = true
            capturing = true
            session.dispatch(CaptureEvent.CaptureStarted)
            val runtime = ViewingRuntime(
                applicationContext,
                auth,
                resultData,
                object : ViewingRuntime.Listener {
                    override fun onState(state: String) {
                        app.viewingState = state
                    }
                    override fun onMediaSas(code: String) {
                        app.mediaSas = code
                    }
                    override fun onPath(path: String) {
                        app.connectionPath = path
                    }
                    override fun onFatal(reason: String) {
                        Handler(mainLooper).post {
                            if (reason == "projection_revoked") {
                                session.dispatch(CaptureEvent.ProjectionRevoked)
                            } else {
                                session.dispatch(CaptureEvent.Failed(reason))
                            }
                            tearDownCapture(notifyStopped = true)
                        }
                    }
                },
            )
            viewing = runtime
            runtime.start()
            return
        }

        val thread = HandlerThread("phonebeam-m0-capture").also { it.start() }
        captureThread = thread
        captureHandler = Handler(thread.looper)

        try {
            val manager = getSystemService(MediaProjectionManager::class.java)
            val projection = manager.getMediaProjection(resultCode, resultData)
                ?: throw IllegalStateException("getMediaProjection returned null")
            mediaProjection = projection
            projection.registerCallback(projectionCallback, Handler(mainLooper))

            val (width, height, dpi) = currentDisplayMetrics()
            attachVirtualDisplay(projection, width, height, dpi)
            displayManager?.registerDisplayListener(displayListener, Handler(mainLooper))
            capturing = true
            windowStartMs = SystemClock.elapsedRealtime()
            session.dispatch(CaptureEvent.CaptureStarted)
        } catch (error: Exception) {
            session.dispatch(
                CaptureEvent.Failed(error.javaClass.simpleName + ": " + (error.message ?: "capture_start_failed")),
            )
            tearDownCapture(notifyStopped = false)
        }
    }

    private fun attachVirtualDisplay(
        projection: MediaProjection,
        width: Int,
        height: Int,
        dpi: Int,
    ) {
        val reader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, IMAGE_BUFFER_COUNT)
        reader.setOnImageAvailableListener({ pending -> drainFrames(pending) }, captureHandler)
        imageReader = reader
        virtualDisplay = projection.createVirtualDisplay(
            VIRTUAL_DISPLAY_NAME,
            width,
            height,
            dpi,
            DisplayManager.VIRTUAL_DISPLAY_FLAG_AUTO_MIRROR,
            reader.surface,
            null,
            captureHandler,
        )
        session.updateStats(
            CaptureStats(
                framesReceived = 0,
                width = width,
                height = height,
                densityDpi = dpi,
            ),
        )
    }

    /**
     * Android 14+ allows only one createVirtualDisplay per MediaProjection token.
     * Rotation must resize and swap the surface, never create a second display.
     */
    private fun resizeCapture(width: Int, height: Int, dpi: Int) {
        val display = virtualDisplay ?: return
        val projection = mediaProjection ?: return
        if (width <= 0 || height <= 0) return
        val current = imageReader
        if (current != null && current.width == width && current.height == height) {
            display.resize(width, height, dpi)
            return
        }
        val reader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, IMAGE_BUFFER_COUNT)
        reader.setOnImageAvailableListener({ pending -> drainFrames(pending) }, captureHandler)
        display.resize(width, height, dpi)
        display.surface = reader.surface
        current?.close()
        imageReader = reader
        session.updateStats(
            session.ui.value.stats.copy(width = width, height = height, densityDpi = dpi),
        )
        // Keep the projection reference used so we never call createVirtualDisplay again.
        check(projection === mediaProjection)
    }

    private fun drainFrames(reader: ImageReader) {
        val image = reader.acquireLatestImage() ?: return
        try {
            framesReceived += 1
            windowFrames += 1
            val now = SystemClock.elapsedRealtime()
            if (now - windowStartMs >= 1_000L) {
                currentFps = windowFrames * 1000f / (now - windowStartMs).coerceAtLeast(1)
                windowStartMs = now
                windowFrames = 0
            }
            val stats = CaptureStats(
                framesReceived = framesReceived,
                width = reader.width,
                height = reader.height,
                densityDpi = currentDensityDpi(),
                framesPerSecond = currentFps,
                lastFrameAtElapsedMs = now,
            )
            Handler(mainLooper).post { session.updateStats(stats) }
            if (framesReceived == 1L || framesReceived - lastPreviewFrame >= PREVIEW_EVERY_N_FRAMES) {
                lastPreviewFrame = framesReceived
                val bitmap = downscaleFrame(image, PREVIEW_MAX_WIDTH)
                if (bitmap != null) {
                    Handler(mainLooper).post { session.updatePreview(bitmap) }
                }
            }
        } finally {
            image.close()
        }
    }

    private fun downscaleFrame(image: Image, maxWidth: Int): Bitmap? {
        val plane = image.planes.firstOrNull() ?: return null
        val width = image.width
        val height = image.height
        if (width <= 0 || height <= 0) return null
        val buffer = plane.buffer
        buffer.rewind()
        val pixelStride = plane.pixelStride
        val rowStride = plane.rowStride
        val rowPadding = rowStride - pixelStride * width
        val full = Bitmap.createBitmap(width + rowPadding / pixelStride, height, Bitmap.Config.ARGB_8888)
        full.copyPixelsFromBuffer(buffer)
        val cropped = if (full.width != width || full.height != height) {
            Bitmap.createBitmap(full, 0, 0, width, height).also { full.recycle() }
        } else {
            full
        }
        if (width <= maxWidth) return cropped
        val previewHeight = (height * (maxWidth.toFloat() / width)).toInt().coerceAtLeast(1)
        val scaled = Bitmap.createScaledBitmap(cropped, maxWidth, previewHeight, true)
        if (scaled !== cropped) {
            cropped.recycle()
        }
        return scaled
    }

    private fun tearDownCapture(notifyStopped: Boolean) {
        capturing = false
        viewing?.stop()
        viewing = null
        (application as? PhoneBeamApp)?.clearViewing()
        displayManager?.unregisterDisplayListener(displayListener)
        virtualDisplay?.release()
        virtualDisplay = null
        imageReader?.close()
        imageReader = null
        mediaProjection?.unregisterCallback(projectionCallback)
        mediaProjection?.stop()
        mediaProjection = null
        captureThread?.quitSafely()
        captureThread = null
        captureHandler = null
        session.clearPreview()
        framesReceived = 0
        lastPreviewFrame = 0
        currentFps = 0f
        stopForeground(STOP_FOREGROUND_REMOVE)
        if (notifyStopped) {
            val state = session.currentState()
            if (state == CaptureState.STOPPING || state == CaptureState.STARTING) {
                session.dispatch(CaptureEvent.StopCompleted)
            }
        }
        stopSelf()
    }

    private fun currentDisplayMetrics(): Triple<Int, Int, Int> {
        val display = displayManager?.getDisplay(Display.DEFAULT_DISPLAY)
        val metrics = DisplayMetrics()
        @Suppress("DEPRECATION")
        display?.getRealMetrics(metrics)
        if (metrics.widthPixels <= 0 || metrics.heightPixels <= 0) {
            val fallback = resources.displayMetrics
            return Triple(fallback.widthPixels, fallback.heightPixels, fallback.densityDpi)
        }
        return Triple(metrics.widthPixels, metrics.heightPixels, metrics.densityDpi)
    }

    private fun currentDensityDpi(): Int = currentDisplayMetrics().third

    private fun createNotificationChannel() {
        val manager = getSystemService(NotificationManager::class.java)
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.capture_notification_channel),
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = getString(R.string.capture_notification_channel_description)
            setShowBadge(false)
        }
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(): Notification {
        val openApp = PendingIntent.getActivity(
            this,
            0,
            Intent(this, if (remoteMode) ViewingActivity::class.java else MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE,
        )
        val stop = PendingIntent.getService(
            this,
            1,
            Intent(this, CaptureService::class.java).setAction(ACTION_STOP),
            PendingIntent.FLAG_IMMUTABLE,
        )
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(
                if (remoteMode) getString(R.string.remote_notification_title) else getString(R.string.capture_notification_title),
            )
            .setContentText(
                if (remoteMode) getString(R.string.remote_notification_text) else getString(R.string.capture_notification_text),
            )
            .setOngoing(true)
            .setSilent(true)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setContentIntent(openApp)
            .addAction(0, getString(R.string.action_stop_share), stop)
            .build()
    }

    companion object {
        const val ACTION_START = "com.phonebeam.android.capture.START"
        const val ACTION_STOP = "com.phonebeam.android.capture.STOP"
        const val EXTRA_RESULT_CODE = "result_code"
        const val EXTRA_RESULT_DATA = "result_data"
        private const val CHANNEL_ID = "phonebeam_capture"
        private const val NOTIFICATION_ID = 1001
        private const val VIRTUAL_DISPLAY_NAME = "phonebeam-m0"
        private const val IMAGE_BUFFER_COUNT = 2
        private const val PREVIEW_EVERY_N_FRAMES = 15L
        private const val PREVIEW_MAX_WIDTH = 240

        fun start(context: Context, resultCode: Int, data: Intent) {
            val intent = Intent(context, CaptureService::class.java).apply {
                action = ACTION_START
                putExtra(EXTRA_RESULT_CODE, resultCode)
                putExtra(EXTRA_RESULT_DATA, data)
            }
            context.startForegroundService(intent)
        }

        fun stop(context: Context) {
            context.startService(Intent(context, CaptureService::class.java).setAction(ACTION_STOP))
        }

        private fun projectionResultIntent(intent: Intent): Intent? {
            return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                intent.getParcelableExtra(EXTRA_RESULT_DATA, Intent::class.java)
            } else {
                @Suppress("DEPRECATION")
                intent.getParcelableExtra(EXTRA_RESULT_DATA)
            }
        }
    }
}
