package docker

import (
	"strings"
	"testing"
)

func TestTailWriterKeepsLastBytesFlattened(t *testing.T) {
	var w tailWriter
	if _, err := w.Write([]byte("  Unable to find image\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := w.Write([]byte("pull access denied  \n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := w.oneLine()
	want := "Unable to find image pull access denied"
	if got != want {
		t.Fatalf("oneLine mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTailWriterCapsRetainedBytes(t *testing.T) {
	var w tailWriter
	if _, err := w.Write([]byte(strings.Repeat("a", maxTail+100))); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := len(w.oneLine()); got != maxTail {
		t.Fatalf("retained %d bytes, want %d", got, maxTail)
	}
}

func TestTailWriterEmpty(t *testing.T) {
	var w tailWriter
	if got := w.oneLine(); got != "" {
		t.Fatalf("oneLine = %q, want empty", got)
	}
}
