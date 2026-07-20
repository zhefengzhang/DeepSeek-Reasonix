package control

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"reasonix/internal/event"
)

// holdFinishingWindow returns a sink that blocks inside the FIRST TurnDone
// delivery until release is closed, holding the controller's finishing window
// deterministically open so tests can place submits inside it. Later
// TurnDones pass through unblocked.
func holdFinishingWindow(release <-chan struct{}, entered chan<- struct{}, events chan<- event.Event) event.Sink {
	var first int32
	return event.FuncSink(func(e event.Event) {
		if e.Kind == event.TurnDone && atomic.AddInt32(&first, 1) == 1 {
			entered <- struct{}{}
			<-release
		}
		if events != nil {
			select {
			case events <- e:
			default:
			}
		}
	})
}

// TestParkedTurnsRunFIFO pins ordering: several submits landing inside one
// finishing window run in arrival order, one per window close, none lost.
func TestParkedTurnsRunFIFO(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	c := New(Options{Sink: holdFinishingWindow(release, entered, nil)})

	c.runGuarded(func(context.Context) error { return nil })
	<-entered

	var order []int
	ran := make(chan int, 3)
	for i := 1; i <= 3; i++ {
		i := i
		c.runGuarded(func(context.Context) error {
			ran <- i
			return nil
		})
	}

	// Verify they were parked (not running yet)
	select {
	case <-ran:
		t.Fatal("parked turn started before the finishing window closed")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	deadline := time.After(30 * time.Second)
	for len(order) < 3 {
		select {
		case i := <-ran:
			order = append(order, i)
		case <-deadline:
			t.Fatalf("parked turns did not all run; got order %v", order)
		}
	}
	if order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("parked turns ran out of order: %v", order)
	}
}

// TestSubmitWhileRunningStaysSilentNoOp pins the running posture: unchanged
// from the historical contract — frontends own the steer/queue UX, internal
// opportunistic callers rely on the quiet no-op.
func TestSubmitWhileRunningStaysSilentNoOp(t *testing.T) {
	block := make(chan struct{})
	started := make(chan struct{})
	c := New(Options{})

	c.runGuarded(func(context.Context) error {
		close(started)
		<-block
		return nil
	})
	<-started

	// Submitting while a turn is running must be a silent no-op
	c.runGuarded(func(context.Context) error {
		t.Fatal("this body should never run")
		return nil
	})

	close(block)
	waitIdleAdmission(t, c)
}

// TestCloseDiscardsParkedTurns pins teardown: a turn parked in the finishing
// window must not start against a controller that has been closed.
func TestCloseDiscardsParkedTurns(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	c := New(Options{Sink: holdFinishingWindow(release, entered, nil)})

	c.runGuarded(func(context.Context) error { return nil })
	<-entered

	parkedRan := make(chan struct{}, 1)
	c.runGuarded(func(context.Context) error {
		parkedRan <- struct{}{}
		return nil
	})

	c.Close()
	close(release)

	select {
	case <-parkedRan:
		t.Fatal("parked turn ran after Close")
	case <-time.After(200 * time.Millisecond):
	}
}

// TestCloseSealsAdmissionDuringFinishingWindow pins the terminal-state
// ordering: Close clears the parked queue, but a submit arriving AFTER that —
// while the old turn's TurnDone delivery is still in flight — must be
// rejected outright, not parked and started against freed resources when the
// window closes.
func TestCloseSealsAdmissionDuringFinishingWindow(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	c := New(Options{Sink: holdFinishingWindow(release, entered, nil)})

	c.runGuarded(func(context.Context) error { return nil })
	<-entered // finishing window is now held open

	c.Close() // seals admission; parked queue is empty at this instant

	lateRan := make(chan struct{}, 1)
	c.runGuarded(func(context.Context) error {
		lateRan <- struct{}{}
		return nil
	})
	// closed gate in runGuarded should silently drop this

	close(release) // window closes; the drain must start nothing
	select {
	case <-lateRan:
		t.Fatal("submit accepted after Close ran when finishing window closed")
	case <-time.After(200 * time.Millisecond):
	}
}

// TestRunTurnRefusedDuringFinishingWindow pins the synchronous gate: RunTurn
// must not start inside the previous turn's TurnDone delivery window — that
// would recreate the completion/transport crosstalk the window prevents.
func TestRunTurnRefusedDuringFinishingWindow(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	c := New(Options{Sink: holdFinishingWindow(release, entered, nil)})

	c.runGuarded(func(context.Context) error { return nil })
	<-entered // finishing window is now held open

	errCh := make(chan error, 1)
	go func() { errCh <- c.RunTurn(context.Background(), "sync input") }()
	select {
	case err := <-errCh:
		if err != ErrTurnRunning {
			t.Fatalf("RunTurn during finishing window = %v, want ErrTurnRunning", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunTurn did not return promptly during the finishing window")
	}
	close(release)
	waitIdleAdmission(t, c)
}

// TestRunTurnRefusedAfterClose pins the terminal state for the synchronous
// entry point too.
func TestRunTurnRefusedAfterClose(t *testing.T) {
	c := New(Options{})
	c.Close()
	if err := c.RunTurn(context.Background(), "late"); err != ErrTurnRunning {
		t.Fatalf("RunTurn after Close = %v, want ErrTurnRunning", err)
	}
}

// waitIdleAdmission polls the running admission gate. A test that submits or
// asserts idle right after TurnDone must wait the finishing window out
// (TurnDone is emitted inside it).
func waitIdleAdmission(t *testing.T, c *Controller) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for c.Running() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the controller to return to idle")
		}
		time.Sleep(time.Millisecond)
	}
}
