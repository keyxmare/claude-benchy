package spec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keyxmare/claude-benchy/internal/spec"
	"gopkg.in/yaml.v3"
)

type importedConfig struct {
	Bundle string `yaml:"bundle"`
}

// importedPaths is the subset of a re-emitted bench.yaml the import tests
// assert on.
type importedPaths struct {
	App    string `yaml:"app"`
	Output string `yaml:"output"`
	Auth   struct {
		ConfigDir string `yaml:"configDir"`
	} `yaml:"auth"`
	Configs []importedConfig `yaml:"configs"`
}

func writeBench(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "bench.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImportAbsolutizesRelativePaths(t *testing.T) {
	t.Parallel()
	path := writeBench(t, `# banc de démonstration
prompt: do it
app: ./app-under-test
output: ./results
auth:
  configDir: ~/.claude
configs:
  - name: baseline
    bundle: ./configs/baseline
  - name: strict
    bundle: /abs/configs/strict
`)
	base := filepath.Dir(path)

	raw, err := spec.Import(path)
	if err != nil {
		t.Fatalf("spec.Import(%q) error = %v", path, err)
	}

	var got importedPaths
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	want := importedPaths{
		App:    filepath.Join(base, "app-under-test"),
		Output: filepath.Join(base, "results"),
		Configs: []importedConfig{
			{Bundle: filepath.Join(base, "configs/baseline")},
			{Bundle: "/abs/configs/strict"},
		},
	}
	want.Auth.ConfigDir = "~/.claude"

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("spec.Import() mismatch (-want +got):\n%s", diff)
	}
}

func TestImportPreservesComments(t *testing.T) {
	t.Parallel()
	path := writeBench(t, `# tête de banc
app: ./app
configs:
  - name: baseline
    bundle: ./b
`)

	raw, err := spec.Import(path)
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if got := string(raw); !strings.Contains(got, "# tête de banc") {
		t.Errorf("spec.Import() dropped comment; output =\n%s", got)
	}
}

func TestImportLeavesEmptyAndConfigsNonSequenceUntouched(t *testing.T) {
	t.Parallel()
	path := writeBench(t, `app: ""
configs: {}
`)

	raw, err := spec.Import(path)
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	var got struct {
		App string `yaml:"app"`
	}
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if got.App != "" {
		t.Errorf("spec.Import() app = %q, want empty", got.App)
	}
	if !strings.Contains(string(raw), "configs: {}") {
		t.Errorf("spec.Import() should leave a non-sequence configs untouched; output =\n%s", raw)
	}
}

func TestImportErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string // set for the missing-file case
		body string // written to a temp file otherwise
		want string
	}{
		{name: "missing file", path: filepath.Join(t.TempDir(), "nope.yaml"), want: "read bench"},
		{name: "invalid yaml", body: "app: [unterminated\n", want: "parse bench"},
		{name: "root not a mapping", body: "- just\n- a list\n", want: "mapping YAML attendu"},
		{name: "empty document", body: "", want: "mapping YAML attendu"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := tt.path
			if path == "" {
				path = writeBench(t, tt.body)
			}

			_, err := spec.Import(path)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("spec.Import() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
