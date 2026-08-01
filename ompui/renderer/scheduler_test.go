package renderer_test

import (
	"sync"
	"testing"
	"time"

	"github.com/lyc-aon/ratatui-go/ompui/component"
	"github.com/lyc-aon/ratatui-go/ompui/renderer"
)

type blockingWriter struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		close(w.started)
		<-w.release
	})
	return len(p), nil
}

func blockingScheduler(t *testing.T) (*renderer.Scheduler, func()) {
	t.Helper()
	writer := &blockingWriter{started: make(chan struct{}), release: make(chan struct{})}
	scheduler := renderer.NewScheduler(renderer.New(writer, renderer.Caps{}), renderer.DefaultScheduler())
	scheduler.RequestImmediate(renderer.Request{
		Frame:  component.NewFrame([]string{"frame"}, 1),
		Width:  20,
		Height: 5,
		Reason: renderer.ReasonForce,
	})
	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("draw did not reach writer")
	}
	var releaseOnce sync.Once
	return scheduler, func() { releaseOnce.Do(func() { close(writer.release) }) }
}

func TestSchedulerSerializesEngineMutationWithDraw(t *testing.T) {
	scheduler, release := blockingScheduler(t)
	defer func() {
		release()
		scheduler.Stop()
	}()

	if rows := scheduler.CommittedRows(); rows != 0 {
		t.Fatalf("committed rows during first draw = %d, want last completed value 0", rows)
	}

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		scheduler.SetCaps(renderer.Caps{SynchronizedOutput: true})
		close(done)
	}()
	<-started
	select {
	case <-done:
		t.Fatal("capability mutation completed during Draw")
	case <-time.After(20 * time.Millisecond):
	}

	release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("capability mutation did not resume after Draw")
	}
}

func TestSchedulerStopWaitsForDraw(t *testing.T) {
	scheduler, release := blockingScheduler(t)
	defer release()

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		scheduler.Stop()
		close(done)
	}()
	<-started
	select {
	case <-done:
		t.Fatal("Stop returned before the active Draw finished")
	case <-time.After(20 * time.Millisecond):
	}

	release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after the active Draw finished")
	}
}
