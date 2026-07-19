package diffcap

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// parseNumstat is unexported and its edge cases — malformed lines and binary
// files git reports as "-" — cannot be driven deterministically through the
// public Capture API, so they are exercised here as an internal invariant.
func TestParseNumstat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		out  string
		want Stats
	}{
		{name: "empty output", out: "", want: Stats{}},
		{
			name: "single file",
			out:  "2\t1\tapp.txt\n",
			want: Stats{FilesChanged: 1, Insertions: 2, Deletions: 1},
		},
		{
			name: "blank interior line is skipped",
			out:  "1\t0\ta.txt\n\n0\t3\tb.txt\n",
			want: Stats{FilesChanged: 2, Insertions: 1, Deletions: 3},
		},
		{
			name: "line with too few fields is skipped",
			out:  "2\t1\ta.txt\nmalformed\n",
			want: Stats{FilesChanged: 1, Insertions: 2, Deletions: 1},
		},
		{
			name: "binary dashes do not parse but the file still counts",
			out:  "-\t-\tbin.dat\n5\t2\tsrc.go\n",
			want: Stats{FilesChanged: 2, Insertions: 5, Deletions: 2},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseNumstat(tt.out)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("parseNumstat(%q) mismatch (-want +got):\n%s", tt.out, d)
			}
		})
	}
}
