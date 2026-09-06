package com.phonebeam.android.capture

/**
 * M0 local capture states. Networking, pairing, and remote control are out of scope.
 */
enum class CaptureState {
    IDLE,
    REQUESTING_PERMISSION,
    STARTING,
    ACTIVE,
    STOPPING,
    STOPPED,
    ERROR,
}

sealed class CaptureEvent {
    data object StartRequested : CaptureEvent()
    data object PermissionGranted : CaptureEvent()
    data object PermissionDenied : CaptureEvent()
    data object CaptureStarted : CaptureEvent()
    data object StopRequested : CaptureEvent()
    data object StopCompleted : CaptureEvent()
    data object ProjectionRevoked : CaptureEvent()
    data class Failed(val reason: String) : CaptureEvent()
}

enum class CaptureEffect {
    None,
    RequestSystemPermission,
    StartCaptureService,
    StopCapture,
}

data class CaptureTransition(
    val from: CaptureState,
    val to: CaptureState,
    val accepted: Boolean,
    val effect: CaptureEffect,
    val errorReason: String? = null,
)

/**
 * Pure M0 capture lifecycle. Android-free so JVM unit tests can prove illegal
 * transitions such as IDLE → ACTIVE without a permission grant.
 */
class CaptureStateMachine(
    initial: CaptureState = CaptureState.IDLE,
) {
    var state: CaptureState = initial
        private set

    var lastError: String? = null
        private set

    fun dispatch(event: CaptureEvent): CaptureTransition {
        val from = state
        val planned = plan(from, event)
        if (planned.accepted) {
            state = planned.to
            lastError = planned.errorReason ?: if (planned.to != CaptureState.ERROR) null else lastError
        }
        return planned
    }

    fun canEnterActiveWithoutPermission(): Boolean = false

    private fun plan(from: CaptureState, event: CaptureEvent): CaptureTransition {
        fun reject(effect: CaptureEffect = CaptureEffect.None) =
            CaptureTransition(from, from, accepted = false, effect = effect)

        fun go(
            to: CaptureState,
            effect: CaptureEffect,
            errorReason: String? = null,
        ) = CaptureTransition(from, to, accepted = true, effect = effect, errorReason = errorReason)

        return when (from) {
            CaptureState.IDLE -> when (event) {
                CaptureEvent.StartRequested ->
                    go(CaptureState.REQUESTING_PERMISSION, CaptureEffect.RequestSystemPermission)
                else -> reject()
            }

            CaptureState.REQUESTING_PERMISSION -> when (event) {
                CaptureEvent.PermissionGranted ->
                    go(CaptureState.STARTING, CaptureEffect.StartCaptureService)
                CaptureEvent.PermissionDenied ->
                    go(CaptureState.STOPPED, CaptureEffect.None)
                CaptureEvent.StopRequested ->
                    go(CaptureState.STOPPED, CaptureEffect.None)
                is CaptureEvent.Failed ->
                    go(CaptureState.ERROR, CaptureEffect.None, event.reason)
                else -> reject()
            }

            CaptureState.STARTING -> when (event) {
                CaptureEvent.CaptureStarted ->
                    go(CaptureState.ACTIVE, CaptureEffect.None)
                CaptureEvent.StopRequested ->
                    go(CaptureState.STOPPING, CaptureEffect.StopCapture)
                CaptureEvent.ProjectionRevoked ->
                    go(CaptureState.STOPPED, CaptureEffect.StopCapture)
                is CaptureEvent.Failed ->
                    go(CaptureState.ERROR, CaptureEffect.StopCapture, event.reason)
                else -> reject()
            }

            CaptureState.ACTIVE -> when (event) {
                CaptureEvent.StopRequested ->
                    go(CaptureState.STOPPING, CaptureEffect.StopCapture)
                CaptureEvent.ProjectionRevoked ->
                    go(CaptureState.STOPPED, CaptureEffect.StopCapture)
                is CaptureEvent.Failed ->
                    go(CaptureState.ERROR, CaptureEffect.StopCapture, event.reason)
                else -> reject()
            }

            CaptureState.STOPPING -> when (event) {
                CaptureEvent.StopCompleted ->
                    go(CaptureState.STOPPED, CaptureEffect.None)
                CaptureEvent.ProjectionRevoked ->
                    go(CaptureState.STOPPED, CaptureEffect.None)
                is CaptureEvent.Failed ->
                    go(CaptureState.ERROR, CaptureEffect.None, event.reason)
                CaptureEvent.StopRequested ->
                    reject()
                else -> reject()
            }

            CaptureState.STOPPED -> when (event) {
                CaptureEvent.StartRequested ->
                    go(CaptureState.REQUESTING_PERMISSION, CaptureEffect.RequestSystemPermission)
                CaptureEvent.StopCompleted ->
                    CaptureTransition(from, from, accepted = true, effect = CaptureEffect.None)
                else -> reject()
            }

            CaptureState.ERROR -> when (event) {
                CaptureEvent.StartRequested ->
                    go(CaptureState.REQUESTING_PERMISSION, CaptureEffect.RequestSystemPermission)
                CaptureEvent.StopCompleted ->
                    CaptureTransition(from, from, accepted = true, effect = CaptureEffect.None)
                else -> reject()
            }
        }
    }
}
