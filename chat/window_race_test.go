package chat

import (
	"context"
	"sync"
	"testing"
	"time"
)

// SetModel re-detects the context window and writes s.compaction.MaxContextTokens
// under modelMu. Every reader of that field must hold the same lock: SetModel
// runs on whichever goroutine drives the UI, while the readers run on the turn
// goroutine, so the two overlap by design — SetModel's own documentation notes
// it "can land at any point inside a turn".
//
// Run with -race. Before the fix, contextWindow released modelMu before reading
// MaxContextTokens and the auto-compaction trigger copied the whole struct with
// no lock at all; the detector reports a write/read race on chat.(*Session).
// compaction. The value is an int and will not tear on the platforms nib
// targets, which is exactly why this needs the detector rather than an
// assertion — there is nothing to assert, only something to observe.
func TestContextWindowDoesNotRaceSetModel(t *testing.T) {
	s := &Session{
		ctx:      context.Background(),
		llmModel: "gpt-4o",
		// A closed port: the capabilities probe fails on connection-refused
		// without a DNS lookup or a round trip, so detectContextSize falls
		// through to the static table quickly. A slow probe would throttle the
		// writer far below the readers and the interleaving this test depends
		// on would become a matter of luck.
		baseURL: "http://127.0.0.1:1/v1",
		// Only an auto-detected window is re-probed on a switch; an explicit
		// user value is preserved, and SetModel would skip the write entirely.
		compactionAutoDetected: true,
	}

	// Both names are in the static table with DIFFERENT windows (128000 and
	// 200000), so each switch genuinely changes the field being read.
	models := []string{"gpt-4o", "o1"}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			s.SetModel(models[i%len(models)])
		}
	}()

	// Several readers, because one writer is far slower than one reader and a
	// single reader makes the overlap rare enough to miss.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				// Both production readers of the window: the display accessor
				// and the auto-compaction trigger.
				//
				// shouldCompactNow rather than shouldAutoCompact(s.compaction,
				// ...) deliberately — reading the struct field straight from a
				// test would reproduce the unguarded access this fix removed,
				// and the detector would then be reporting the test's bug
				// instead of the code's.
				_ = s.ContextWindow()
				_ = s.shouldCompactNow(1)
			}
		}()
	}

	time.Sleep(250 * time.Millisecond)
	close(stop)
	wg.Wait()
}
