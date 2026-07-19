package server

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/docker"
)

type nopDocker struct{}

func (nopDocker) Build(context.Context, string, string, string, map[string]string) error { return nil }
func (nopDocker) Run(context.Context, docker.RunSpec, io.Writer, io.Writer) error        { return nil }

func TestResolveExistingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "bench.yaml")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, err := New(root, "img", nopDocker{})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty", in: "  ", wantErr: true},
		{name: "relative existing", in: "./bench.yaml", want: file},
		{name: "absolute existing", in: file, want: file},
		{name: "directory", in: "./app", wantErr: true},
		{name: "missing", in: "./nope.yaml", wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := srv.resolveExistingFile(tt.in)

			if tt.wantErr {
				if err == nil {
					t.Errorf("resolveExistingFile(%q) error = nil, want error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveExistingFile(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("resolveExistingFile(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
