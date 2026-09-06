package com.phonebeam.android.capture

import android.graphics.Bitmap
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update

data class CaptureStats(
    val framesReceived: Long = 0,
    val width: Int = 0,
    val height: Int = 0,
    val densityDpi: Int = 0,
    val framesPerSecond: Float = 0f,
    val lastFrameAtElapsedMs: Long = 0,
)

data class CaptureUiState(
    val state: CaptureState = CaptureState.IDLE,
    val stats: CaptureStats = CaptureStats(),
    val preview: Bitmap? = null,
    val errorReason: String? = null,
    val stopReason: String? = null,
)

/**
 * Process-wide M0 session. The Activity and [CaptureService] dispatch events here.
 * It never starts capture by itself and never caches a MediaProjection result Intent.
 */
class CaptureSession {
    private val machine = CaptureStateMachine()
    private val _ui = MutableStateFlow(CaptureUiState())
    val ui: StateFlow<CaptureUiState> = _ui.asStateFlow()

    @Synchronized
    fun dispatch(event: CaptureEvent): CaptureTransition {
        val transition = machine.dispatch(event)
        if (transition.accepted) {
            _ui.update { current ->
                current.copy(
                    state = transition.to,
                    errorReason = when (transition.to) {
                        CaptureState.ERROR -> transition.errorReason ?: current.errorReason
                        CaptureState.REQUESTING_PERMISSION, CaptureState.STARTING, CaptureState.ACTIVE -> null
                        else -> current.errorReason
                    },
                    stopReason = when {
                        event is CaptureEvent.PermissionDenied -> "permission_denied"
                        event is CaptureEvent.ProjectionRevoked -> "projection_revoked"
                        event is CaptureEvent.StopRequested -> "user_stop"
                        transition.to == CaptureState.IDLE -> null
                        else -> current.stopReason
                    },
                    stats = if (transition.to == CaptureState.REQUESTING_PERMISSION) CaptureStats() else current.stats,
                    preview = if (transition.to == CaptureState.REQUESTING_PERMISSION) null else current.preview,
                )
            }
        }
        return transition
    }

    fun currentState(): CaptureState = machine.state

    fun updateStats(stats: CaptureStats) {
        _ui.update { it.copy(stats = stats) }
    }

    fun updatePreview(bitmap: Bitmap?) {
        _ui.update { it.copy(preview = bitmap) }
    }

    fun clearPreview() {
        _ui.update { it.copy(preview = null) }
    }
}
