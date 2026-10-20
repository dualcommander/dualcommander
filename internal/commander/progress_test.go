package commander

import (
	"sync"
	"testing"
)

func TestProgressQueueBounded(t *testing.T) {
	queue := make(chan func(), 2)
	var gotDone, gotTotal int64
	var gotFile string
	progress := latestProgress(func(f func()) { queue <- f }, func(done, total int64, file string) {
		gotDone, gotTotal, gotFile = done, total, file
	})
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := int64(1); i <= 10000; i++ {
				progress(i, 10000, "intermediate")
			}
		}()
	}
	workers.Wait()
	progress(10000, 10000, "latest")
	if got := len(queue); got != 1 {
		t.Fatalf("queued %d updates; want one latest-value update", got)
	}
	(<-queue)()
	if gotDone != 10000 || gotTotal != 10000 || gotFile != "latest" {
		t.Fatalf("stale progress: %d/%d %q", gotDone, gotTotal, gotFile)
	}
	progress(1, 2, "next")
	if len(queue) != 1 {
		t.Fatal("draining progress did not permit next update")
	}
	(<-queue)()
	if gotDone != 1 || gotTotal != 2 || gotFile != "next" {
		t.Fatal("next progress lost")
	}
}
