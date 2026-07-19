package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type importedConfig struct {
	Bundle     string `yaml:"bundle"`
	PromptFile string `yaml:"promptFile"`
}

// importedPaths is the subset of a re-emitted bench.yaml the import tests
// assert on.
type importedPaths struct {
	App        string `yaml:"app"`
	Output     string `yaml:"output"`
	PromptFile string `yaml:"promptFile"`
	Auth       struct {
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
promptFile: ./prompts/x.md
auth:
  configDir: ~/.claude
configs:
  - name: baseline
    bundle: ./configs/baseline
  - name: strict
    bundle: /abs/configs/strict
    promptFile: ./p.md
`)
	base := filepath.Dir(path)

	raw, err := Import(path)
	if err != nil {
		t.Fatalf("Import(%q) error = %v", path, err)
	}

	var got importedPaths
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	want := importedPaths{
		App:        filepath.Join(base, "app-under-test"),
		Output:     filepath.Join(base, "results"),
		PromptFile: filepath.Join(base, "prompts/x.md"),
		Configs: []importedConfig{
			{Bundle: filepath.Join(base, "configs/baseline")},
			{Bundle: "/abs/configs/strict", PromptFile: filepath.Join(base, "p.md")},
		},
	}
	want.Auth.ConfigDir = "~/.claude"

	if got.App != want.App || got.Output != want.Output || got.PromptFile != want.PromptFile || got.Auth.ConfigDir != want.Auth.ConfigDir {
		t.Errorf("Import() scalars = %+v, want %+v", got, want)
	}
	if len(got.Configs) != len(want.Configs) {
		t.Fatalf("Import() configs = %+v, want %+v", got.Configs, want.Configs)
	}
	for i, wc := range want.Configs {
		if got.Configs[i] != wc {
			t.Errorf("Import() config[%d] = %+v, want %+v", i, got.Configs[i], wc)
		}
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

	raw, err := Import(path)
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if got := string(raw); !strings.Contains(got, "# tête de banc") {
		t.Errorf("Import() dropped comment; output =\n%s", got)
	}
}

func TestImportLeavesEmptyAndConfigsNonSequenceUntouched(t *testing.T) {
	t.Parallel()
	// app empty, output absent, configs not a sequence: no path is rewritten.
	path := writeBench(t, `app: ""
configs: {}
`)

	raw, err := Import(path)
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
		t.Errorf("Import() app = %q, want empty", got.App)
	}
	if !strings.Contains(string(raw), "configs: {}") {
		t.Errorf("Import() should leave a non-sequence configs untouched; output =\n%s", raw)
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

			_, err := Import(path)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Import() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestDocumentMapping(t *testing.T) {
	t.Parallel()
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "x"}
	tests := []struct {
		name string
		in   *yaml.Node
		want *yaml.Node
	}{
		{name: "document wrapping mapping", in: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}, want: mapping},
		{name: "bare mapping", in: mapping, want: mapping},
		{name: "document with two children", in: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping, scalar}}, want: nil},
		{name: "scalar", in: scalar, want: nil},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := documentMapping(tt.in); got != tt.want {
				t.Errorf("documentMapping(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestMapValue(t *testing.T) {
	t.Parallel()
	value := &yaml.Node{Kind: yaml.ScalarNode, Value: "v"}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "k"}, value,
	}}
	tests := []struct {
		name string
		m    *yaml.Node
		key  string
		want *yaml.Node
	}{
		{name: "nil node", m: nil, key: "k", want: nil},
		{name: "not a mapping", m: value, key: "k", want: nil},
		{name: "key present", m: mapping, key: "k", want: value},
		{name: "key absent", m: mapping, key: "other", want: nil},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mapValue(tt.m, tt.key); got != tt.want {
				t.Errorf("mapValue(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestAbsolutizeScalar(t *testing.T) {
	t.Parallel()
	base := "/base"
	tests := []struct {
		name string
		in   *yaml.Node
		want string
	}{
		{name: "nil", in: nil, want: ""},
		{name: "not a scalar", in: &yaml.Node{Kind: yaml.MappingNode}, want: ""},
		{name: "empty", in: &yaml.Node{Kind: yaml.ScalarNode, Value: ""}, want: ""},
		{name: "already absolute", in: &yaml.Node{Kind: yaml.ScalarNode, Value: "/x/y"}, want: "/x/y"},
		{name: "home based", in: &yaml.Node{Kind: yaml.ScalarNode, Value: "~/z"}, want: "~/z"},
		{name: "relative", in: &yaml.Node{Kind: yaml.ScalarNode, Value: "./a/b"}, want: "/base/a/b"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			absolutizeScalar(tt.in, base)
			got := ""
			if tt.in != nil {
				got = tt.in.Value
			}
			if got != tt.want {
				t.Errorf("absolutizeScalar(%s).Value = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
