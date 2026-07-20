package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/keyxmare/claude-benchy/internal/spec"
)

// formValues holds the raw dashboard form fields. It is both the parse target
// and the model used to repopulate the form when validation fails.
type formValues struct {
	Prompt         string
	App            string
	Model          string
	Runs           string
	Concurrency    string
	Retries        string
	AuthConfigDir  string
	SandboxImage   string
	Output         string
	EvalModel      string
	Rubric         string
	KeepBaseConfig bool
	Configs        []configRow
	Checks         []checkRow
}

type configRow struct{ Name, Bundle, Model, Prompt string }

type checkRow struct{ Name, Run, File string }

// defaultForm is the pristine form shown on the dashboard.
func defaultForm() formValues {
	return formValues{
		Model:       "sonnet",
		Runs:        "1",
		Concurrency: "1",
		Retries:     "2",
		Output:      "results",
		Configs:     []configRow{{}},
	}
}

// readForm extracts the submitted fields. Config and check rows share a name
// across inputs so they arrive as parallel slices aligned by index.
func readForm(r *http.Request) formValues {
	_ = r.ParseForm()
	fv := formValues{
		Prompt:         r.FormValue("prompt"),
		App:            r.FormValue("app"),
		Model:          r.FormValue("model"),
		Runs:           r.FormValue("runs"),
		Concurrency:    r.FormValue("concurrency"),
		Retries:        r.FormValue("retries"),
		AuthConfigDir:  r.FormValue("authConfigDir"),
		SandboxImage:   r.FormValue("sandboxImage"),
		Output:         r.FormValue("output"),
		EvalModel:      r.FormValue("evalModel"),
		Rubric:         r.FormValue("rubric"),
		KeepBaseConfig: r.FormValue("keepBaseConfig") != "",
	}

	names, bundles := r.Form["configName"], r.Form["configBundle"]
	models, prompts := r.Form["configModel"], r.Form["configPrompt"]
	for i := range names {
		fv.Configs = append(fv.Configs, configRow{
			Name:   at(names, i),
			Bundle: at(bundles, i),
			Model:  at(models, i),
			Prompt: at(prompts, i),
		})
	}
	if len(fv.Configs) == 0 {
		fv.Configs = []configRow{{}}
	}

	cn, cr, cf := r.Form["checkName"], r.Form["checkRun"], r.Form["checkFile"]
	for i := range cn {
		fv.Checks = append(fv.Checks, checkRow{Name: at(cn, i), Run: at(cr, i), File: at(cf, i)})
	}
	return fv
}

// spec assembles an unresolved spec.Spec from the form. Path resolution and the
// substantive validation happen later in spec.Build; here we only parse the
// numeric fields and drop blank repeated rows.
func (fv formValues) spec() (spec.Spec, error) {
	runs, err := atoiField("runs", fv.Runs)
	if err != nil {
		return spec.Spec{}, err
	}
	conc, err := atoiField("concurrency", fv.Concurrency)
	if err != nil {
		return spec.Spec{}, err
	}
	retries, err := atoiPtrField("retries", fv.Retries)
	if err != nil {
		return spec.Spec{}, err
	}

	s := spec.Spec{
		Prompt:         blankToEmpty(fv.Prompt),
		App:            strings.TrimSpace(fv.App),
		Model:          strings.TrimSpace(fv.Model),
		Runs:           runs,
		Concurrency:    conc,
		Retries:        retries,
		Output:         strings.TrimSpace(fv.Output),
		Auth:           spec.Auth{ConfigDir: strings.TrimSpace(fv.AuthConfigDir)},
		Sandbox:        spec.Sandbox{Image: strings.TrimSpace(fv.SandboxImage)},
		KeepBaseConfig: fv.KeepBaseConfig,
		Evaluate: spec.Evaluate{
			Model:  strings.TrimSpace(fv.EvalModel),
			Rubric: splitLines(fv.Rubric),
		},
	}
	for _, c := range fv.Configs {
		if isBlank(c.Name, c.Bundle, c.Model, c.Prompt) {
			continue
		}
		s.Configs = append(s.Configs, spec.Config{
			Name:   strings.TrimSpace(c.Name),
			Bundle: strings.TrimSpace(c.Bundle),
			Model:  strings.TrimSpace(c.Model),
			Prompt: blankToEmpty(c.Prompt),
		})
	}
	for _, ck := range fv.Checks {
		if isBlank(ck.Name, ck.Run, ck.File) {
			continue
		}
		s.Evaluate.Checks = append(s.Evaluate.Checks, spec.Check{
			Name: strings.TrimSpace(ck.Name),
			Run:  strings.TrimSpace(ck.Run),
			File: strings.TrimSpace(ck.File),
		})
	}
	return s, nil
}

// benchDoc mirrors the bench.yaml layout with omitempty so the persisted file
// stays tidy — it is the serialization boundary, kept separate from spec.Spec.
type benchDoc struct {
	Prompt         string       `yaml:"prompt,omitempty"`
	App            string       `yaml:"app"`
	Model          string       `yaml:"model,omitempty"`
	Runs           int          `yaml:"runs,omitempty"`
	Concurrency    int          `yaml:"concurrency,omitempty"`
	Retries        *int         `yaml:"retries,omitempty"`
	Auth           *authDoc     `yaml:"auth,omitempty"`
	Sandbox        *sandboxDoc  `yaml:"sandbox,omitempty"`
	Output         string       `yaml:"output,omitempty"`
	KeepBaseConfig bool         `yaml:"keepBaseConfig,omitempty"`
	Evaluate       *evaluateDoc `yaml:"evaluate,omitempty"`
	Configs        []configDoc  `yaml:"configs"`
}

type authDoc struct {
	ConfigDir string `yaml:"configDir,omitempty"`
}

type sandboxDoc struct {
	Image string `yaml:"image,omitempty"`
}

type evaluateDoc struct {
	Model  string     `yaml:"model,omitempty"`
	Rubric []string   `yaml:"rubric,omitempty"`
	Checks []checkDoc `yaml:"checks,omitempty"`
}

type checkDoc struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run,omitempty"`
	File string `yaml:"file,omitempty"`
}

type configDoc struct {
	Name   string `yaml:"name"`
	Bundle string `yaml:"bundle"`
	Model  string `yaml:"model,omitempty"`
	Prompt string `yaml:"prompt,omitempty"`
}

// benchDocFrom builds the persistable document from an unresolved spec.
func benchDocFrom(s spec.Spec) benchDoc {
	d := benchDoc{
		Prompt:         s.Prompt,
		App:            s.App,
		Model:          s.Model,
		Runs:           s.Runs,
		Concurrency:    s.Concurrency,
		Retries:        s.Retries,
		Output:         s.Output,
		KeepBaseConfig: s.KeepBaseConfig,
	}
	if s.Auth.ConfigDir != "" {
		d.Auth = &authDoc{ConfigDir: s.Auth.ConfigDir}
	}
	if s.Sandbox.Image != "" {
		d.Sandbox = &sandboxDoc{Image: s.Sandbox.Image}
	}
	if s.Evaluate.Model != "" || s.Evaluate.Enabled() {
		ed := &evaluateDoc{Model: s.Evaluate.Model, Rubric: s.Evaluate.Rubric}
		for _, c := range s.Evaluate.Checks {
			ed.Checks = append(ed.Checks, checkDoc{Name: c.Name, Run: c.Run, File: c.File})
		}
		d.Evaluate = ed
	}
	for _, c := range s.Configs {
		d.Configs = append(d.Configs, configDoc{Name: c.Name, Bundle: c.Bundle, Model: c.Model, Prompt: c.Prompt})
	}
	return d
}

func atoiField(name, v string) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s : nombre invalide %q", name, v)
	}
	return n, nil
}

// atoiPtrField parses an optional integer: a blank field yields nil (use the
// default) while an explicit value — including 0 — is preserved.
func atoiPtrField(name, v string) (*int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil, fmt.Errorf("%s : nombre invalide %q", name, v)
	}
	return &n, nil
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func blankToEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s
}

func isBlank(fields ...string) bool {
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}
