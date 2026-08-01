package component_test

import (
	"sync"
	"testing"

	"github.com/lyc-aon/ratatui-go/ompui/component"
)

func TestCommittedRowsConcurrentWithRender(t *testing.T) {
	leaf := component.NewRemote("leaf")
	leaf.SetLines([]string{"row"})
	root := component.NewContainer(leaf)

	start := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		<-start
		for i := 0; i < 10_000; i++ {
			root.SetNativeScrollbackCommittedRows(i)
			leaf.SetNativeScrollbackCommittedRows(i)
		}
	}()

	close(start)
	for i := 0; i < 10_000; i++ {
		if frame := root.Render(40); len(frame.Lines) != 1 {
			t.Fatalf("rendered rows = %d, want 1", len(frame.Lines))
		}
		_ = leaf.CommittedRows()
	}
	writer.Wait()

	root.SetNativeScrollbackCommittedRows(-1)
	root.Render(40)
	if got := leaf.CommittedRows(); got != 0 {
		t.Fatalf("negative committed rows propagated as %d, want 0", got)
	}
}
