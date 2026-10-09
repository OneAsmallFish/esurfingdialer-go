package state

import (
	"sync"
	"testing"
)

func TestDoneClosesWhenRunningStops(t *testing.T) {
	st := New()
	select {
	case <-st.Done():
		t.Fatal("Done closed while state is running")
	default:
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st.SetRunning(false)
		}()
	}
	wg.Wait()
	select {
	case <-st.Done():
	default:
		t.Fatal("Done remained open after state stopped")
	}

	st.SetRunning(false)
	st.SetRunning(true)
	if st.IsRunning() {
		t.Fatal("stopped state restarted with a closed Done channel")
	}
}
