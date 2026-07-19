package server

import (
	"path/filepath"
	"testing"
)

func TestReserveDirSuffixesOnCollision(t *testing.T) {
	// Two reservations landing in the same wall-clock second must resolve to
	// distinct directories, the second gaining a "-2" suffix. A fresh output
	// directory per attempt keeps the expected suffix exact; the loop simply
	// retries across the rare second boundary.
	for attempt := 0; attempt < 100; attempt++ {
		out := t.TempDir()
		first, err := reserveDir(out)
		if err != nil {
			t.Fatalf("first reserveDir: %v", err)
		}
		second, err := reserveDir(out)
		if err != nil {
			t.Fatalf("second reserveDir: %v", err)
		}
		if filepath.Base(second) == filepath.Base(first)+"-2" {
			return
		}
	}
	t.Fatal("never observed a same-second reservation collision")
}
