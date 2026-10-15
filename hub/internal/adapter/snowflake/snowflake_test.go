package snowflake

import (
	"testing"
	"time"
)

type stepClock struct{ now time.Time }

func (c *stepClock) Now() time.Time { return c.now }

func TestNewIDIsMonotonic(t *testing.T) {
	clock := &stepClock{now: epoch.Add(time.Hour)}
	g, err := New(clock, 3)
	if err != nil {
		t.Fatal(err)
	}

	var last int64
	for i := range 10_000 {
		switch i {
		case 5_000:
			clock.now = clock.now.Add(-time.Second) // clock steps backwards
		case 7_000:
			clock.now = clock.now.Add(time.Minute)
		}
		id := int64(g.NewID())
		if id <= last {
			t.Fatalf("id %d at step %d is not greater than %d", id, i, last)
		}
		if worker := id >> seqBits & maxWorker; worker != 3 {
			t.Fatalf("worker bits = %d, want 3", worker)
		}
		last = id
	}
}

func TestNewIDEncodesTime(t *testing.T) {
	at := epoch.Add(42 * time.Millisecond)
	g, err := New(&stepClock{now: at}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ms := int64(g.NewID()) >> (workerBits + seqBits); ms != 42 {
		t.Errorf("timestamp bits = %d, want 42", ms)
	}
}

func TestNewRejectsWorkerOutOfRange(t *testing.T) {
	for _, w := range []int{-1, maxWorker + 1} {
		if _, err := New(&stepClock{}, w); err == nil {
			t.Errorf("worker %d accepted", w)
		}
	}
}
