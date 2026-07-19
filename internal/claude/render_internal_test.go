package claude

import (
	"strings"
	"testing"
)

func TestTruncateRuneSafe(t *testing.T) {
	if got := truncate(strings.Repeat("é", 5), 3); got != "ééé…" {
		t.Fatalf("truncate = %q", got)
	}
}
