package com.phonebeam.android.capture

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class CaptureStateMachineTest {

    @Test
    fun startsIdle() {
        val machine = CaptureStateMachine()
        assertEquals(CaptureState.IDLE, machine.state)
    }

    @Test
    fun startRequestedMovesToRequestingPermission() {
        val machine = CaptureStateMachine()
        val transition = machine.dispatch(CaptureEvent.StartRequested)
        assertTrue(transition.accepted)
        assertEquals(CaptureState.REQUESTING_PERMISSION, machine.state)
        assertEquals(CaptureEffect.RequestSystemPermission, transition.effect)
    }

    @Test
    fun permissionDeniedStopsWithoutActivating() {
        val machine = CaptureStateMachine()
        machine.dispatch(CaptureEvent.StartRequested)
        val transition = machine.dispatch(CaptureEvent.PermissionDenied)
        assertTrue(transition.accepted)
        assertEquals(CaptureState.STOPPED, machine.state)
        assertEquals(CaptureEffect.None, transition.effect)
    }

    @Test
    fun permissionGrantedStartsServiceNotActive() {
        val machine = CaptureStateMachine()
        machine.dispatch(CaptureEvent.StartRequested)
        val transition = machine.dispatch(CaptureEvent.PermissionGranted)
        assertTrue(transition.accepted)
        assertEquals(CaptureState.STARTING, machine.state)
        assertEquals(CaptureEffect.StartCaptureService, transition.effect)
        assertFalse(machine.state == CaptureState.ACTIVE)
    }

    @Test
    fun captureStartedReachesActive() {
        val machine = grantedAndStarting()
        val transition = machine.dispatch(CaptureEvent.CaptureStarted)
        assertTrue(transition.accepted)
        assertEquals(CaptureState.ACTIVE, machine.state)
    }

    @Test
    fun stopFromActiveGoesStoppingThenStopped() {
        val machine = active()
        val stop = machine.dispatch(CaptureEvent.StopRequested)
        assertEquals(CaptureState.STOPPING, stop.to)
        assertEquals(CaptureEffect.StopCapture, stop.effect)
        val done = machine.dispatch(CaptureEvent.StopCompleted)
        assertEquals(CaptureState.STOPPED, done.to)
    }

    @Test
    fun projectionRevokedFromActiveFailsClosed() {
        val machine = active()
        val transition = machine.dispatch(CaptureEvent.ProjectionRevoked)
        assertTrue(transition.accepted)
        assertEquals(CaptureState.STOPPED, machine.state)
        assertEquals(CaptureEffect.StopCapture, transition.effect)
    }

    @Test
    fun failedDuringStartCleansUp() {
        val machine = grantedAndStarting()
        val transition = machine.dispatch(CaptureEvent.Failed("virtual_display"))
        assertEquals(CaptureState.ERROR, machine.state)
        assertEquals(CaptureEffect.StopCapture, transition.effect)
        assertEquals("virtual_display", transition.errorReason)
    }

    @Test
    fun idleCannotJumpToActive() {
        val machine = CaptureStateMachine()
        val started = machine.dispatch(CaptureEvent.CaptureStarted)
        val granted = machine.dispatch(CaptureEvent.PermissionGranted)
        assertFalse(started.accepted)
        assertFalse(granted.accepted)
        assertEquals(CaptureState.IDLE, machine.state)
        assertFalse(machine.canEnterActiveWithoutPermission())
    }

    @Test
    fun stoppedRequiresNewPermissionToCaptureAgain() {
        val machine = active()
        machine.dispatch(CaptureEvent.StopRequested)
        machine.dispatch(CaptureEvent.StopCompleted)
        val again = machine.dispatch(CaptureEvent.CaptureStarted)
        assertFalse(again.accepted)
        assertEquals(CaptureState.STOPPED, machine.state)
        val restart = machine.dispatch(CaptureEvent.StartRequested)
        assertEquals(CaptureState.REQUESTING_PERMISSION, restart.to)
        assertEquals(CaptureEffect.RequestSystemPermission, restart.effect)
    }

    @Test
    fun errorAllowsRestartOnlyThroughPermission() {
        val machine = CaptureStateMachine()
        machine.dispatch(CaptureEvent.StartRequested)
        machine.dispatch(CaptureEvent.Failed("denied_notifications"))
        assertEquals(CaptureState.ERROR, machine.state)
        assertFalse(machine.dispatch(CaptureEvent.CaptureStarted).accepted)
        assertEquals(CaptureEffect.RequestSystemPermission, machine.dispatch(CaptureEvent.StartRequested).effect)
    }

    private fun grantedAndStarting(): CaptureStateMachine {
        val machine = CaptureStateMachine()
        machine.dispatch(CaptureEvent.StartRequested)
        machine.dispatch(CaptureEvent.PermissionGranted)
        return machine
    }

    private fun active(): CaptureStateMachine {
        val machine = grantedAndStarting()
        machine.dispatch(CaptureEvent.CaptureStarted)
        return machine
    }
}
