package spec

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigDir(t *testing.T) {
	tests := []struct {
		name       string
		configEnv  string
		homeEnv    string
		wantEnv    bool // want == configEnv
		wantFallbk bool // want == ".claude"
	}{
		{name: "CLAUDE_CONFIG_DIR wins", configEnv: "/custom/claude", homeEnv: "/home/tester", wantEnv: true},
		{name: "falls back to home/.claude", configEnv: "", homeEnv: "/home/tester"},
		{name: "no home falls back to .claude", configEnv: "", homeEnv: "", wantFallbk: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CONFIG_DIR", tt.configEnv)
			t.Setenv("HOME", tt.homeEnv)

			got := defaultConfigDir()

			var want string
			switch {
			case tt.wantEnv:
				want = tt.configEnv
			case tt.wantFallbk:
				want = ".claude"
			default:
				want = filepath.Join(tt.homeEnv, ".claude")
			}
			if got != want {
				t.Errorf("defaultConfigDir() = %q, want %q", got, want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	const home = "/home/tester"
	tests := []struct {
		name    string
		in      string
		homeEnv string
		want    string
		wantErr bool
	}{
		{name: "tilde alone", in: "~", homeEnv: home, want: home},
		{name: "tilde slash path", in: "~/x/y", homeEnv: home, want: filepath.Join(home, "x/y")},
		{name: "no tilde stays absolute", in: "/etc/app", homeEnv: home, want: "/etc/app"},
		{name: "tilde without home errors", in: "~/x", homeEnv: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", tt.homeEnv)

			got, err := expandHome(tt.in)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expandHome(%q) error = nil, want error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("expandHome(%q) error = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
	// The filepath.Abs failure branch (a relative path when os.Getwd fails) is
	// unreachable in a test: Getwd cannot be made to fail deterministically.
}

func TestExpandHomeRelativeResolvesAgainstCwd(t *testing.T) {
	got, err := expandHome("rel/path")
	if err != nil {
		t.Fatalf("expandHome(rel) error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expandHome(rel) = %q, want absolute", got)
	}
}
