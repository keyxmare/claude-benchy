package server

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFormSpecDropsBlankRowsAndParsesLists(t *testing.T) {
	fv := formValues{
		Prompt:      "do it",
		App:         "./app",
		Model:       "sonnet",
		Runs:        "2",
		Concurrency: "1",
		Output:      "results",
		Rubric:      "critère un\n\n  critère deux  \n",
		Configs: []configRow{
			{Name: "baseline", Bundle: "./configs/baseline"},
			{}, // blank row, dropped
			{Name: "strict", Bundle: "./configs/strict", Model: "opus"},
		},
		Checks: []checkRow{
			{Name: "gate", Run: "go test ./..."},
			{}, // blank row, dropped
		},
	}

	s, err := fv.spec()
	if err != nil {
		t.Fatal(err)
	}
	if s.Runs != 2 {
		t.Errorf("runs = %d, want 2", s.Runs)
	}
	if len(s.Configs) != 2 || s.Configs[1].Name != "strict" || s.Configs[1].Model != "opus" {
		t.Errorf("configs not parsed / compacted: %+v", s.Configs)
	}
	if len(s.Evaluate.Rubric) != 2 || s.Evaluate.Rubric[1] != "critère deux" {
		t.Errorf("rubric not split/trimmed: %v", s.Evaluate.Rubric)
	}
	if len(s.Evaluate.Checks) != 1 || s.Evaluate.Checks[0].Name != "gate" {
		t.Errorf("checks not parsed / compacted: %+v", s.Evaluate.Checks)
	}
}

func TestFormCarriesKeepBaseConfig(t *testing.T) {
	fv := formValues{
		App: "./app", KeepBaseConfig: true,
		Configs: []configRow{{Name: "baseline", Bundle: "./b"}},
	}
	s, err := fv.spec()
	if err != nil {
		t.Fatal(err)
	}
	if !s.KeepBaseConfig {
		t.Error("keepBaseConfig should flow into the spec")
	}
	out, err := yaml.Marshal(benchDocFrom(s))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "keepBaseConfig: true") {
		t.Errorf("bench.yaml should record keepBaseConfig, got:\n%s", out)
	}
}

func TestFormRetriesRoundTrip(t *testing.T) {
	cases := map[string]struct {
		field   string
		wantNil bool
		wantVal int
		wantYML string // substring expected in the persisted bench.yaml
	}{
		"blank uses default": {"", true, 0, ""},
		"explicit zero":      {"0", false, 0, "retries: 0"},
		"explicit number":    {"3", false, 3, "retries: 3"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fv := formValues{App: "./app", Retries: tc.field, Configs: []configRow{{Name: "a", Bundle: "./b"}}}
			s, err := fv.spec()
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantNil {
				if s.Retries != nil {
					t.Errorf("expected nil retries, got %v", *s.Retries)
				}
			} else if s.Retries == nil || *s.Retries != tc.wantVal {
				t.Errorf("retries = %v, want %d", s.Retries, tc.wantVal)
			}

			out, err := yaml.Marshal(benchDocFrom(s))
			if err != nil {
				t.Fatal(err)
			}
			got := string(out)
			if tc.wantYML == "" {
				if strings.Contains(got, "retries:") {
					t.Errorf("blank retries should be omitted, got:\n%s", got)
				}
			} else if !strings.Contains(got, tc.wantYML) {
				t.Errorf("bench.yaml missing %q, got:\n%s", tc.wantYML, got)
			}

			// The persisted value reopens in the form unchanged.
			if back := formFromDoc(benchDocFrom(s)).Retries; !tc.wantNil && back != tc.field {
				t.Errorf("reused form retries = %q, want %q", back, tc.field)
			}
		})
	}
}

func TestChooseImagePrefersBenchImage(t *testing.T) {
	if got := chooseImage("claude-benchy:latest", "claude-benchy-go:latest"); got != "claude-benchy-go:latest" {
		t.Errorf("bench sandbox.image should win, got %q", got)
	}
	if got := chooseImage("claude-benchy:latest", ""); got != "claude-benchy:latest" {
		t.Errorf("empty bench image should fall back to the server default, got %q", got)
	}
}

func TestFormSpecRejectsInvalidNumber(t *testing.T) {
	fv := defaultForm()
	fv.Runs = "three"
	if _, err := fv.spec(); err == nil {
		t.Fatal("expected an error for a non-numeric runs field")
	}
}

func TestBenchDocMarshalsTidyYAML(t *testing.T) {
	fv := formValues{
		Prompt: "do it", App: "./app", Model: "sonnet", Output: "results",
		Configs: []configRow{{Name: "baseline", Bundle: "./configs/baseline"}},
	}
	s, err := fv.spec()
	if err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(benchDocFrom(s))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	// omitempty keeps the persisted bench readable: no empty auth/sandbox/evaluate.
	for _, unwanted := range []string{"auth:", "sandbox:", "evaluate:", "promptFile:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("bench.yaml should omit empty %q, got:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, "app: ./app") || !strings.Contains(got, "name: baseline") {
		t.Errorf("bench.yaml missing core fields:\n%s", got)
	}
}
