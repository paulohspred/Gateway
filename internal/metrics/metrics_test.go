package metrics

import (
	"sync"
	"testing"
	"time"
)

type blockingWriter struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		close(w.entered)
		<-w.release
	})
	return len(p), nil
}

func TestWritePrometheusDoesNotBlockDataPlaneUpdates(t *testing.T) {
	r := New()
	r.Inc("rc_gateway_test_total")

	w := &blockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		r.WritePrometheus(w)
		close(done)
	}()

	select {
	case <-w.entered:
	case <-time.After(time.Second):
		t.Fatal("metrics writer did not start")
	}

	updated := make(chan struct{})
	go func() {
		r.Inc("rc_gateway_data_plane_total")
		close(updated)
	}()

	select {
	case <-updated:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("registry update blocked on slow metrics client")
	}

	close(w.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("metrics writer did not finish")
	}
}
