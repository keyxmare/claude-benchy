package spec

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCredsFileDarwinExportsKeychain(t *testing.T) {
	dir := t.TempDir()
	restore := swapCreds(t, "darwin", func() ([]byte, error) { return []byte("{\"claudeAiOauth\":{}}"), nil })
	defer restore()

	got := resolveCredsFile(dir)

	want := filepath.Join(dir, credentialsFile)
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read exported creds: %v", err)
	}
	if string(data) != "{\"claudeAiOauth\":{}}" {
		t.Fatalf("exported content = %q", data)
	}
	if info, _ := os.Stat(want); info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %v, want 0600", info.Mode().Perm())
	}
}

func TestResolveCredsFileDarwinKeychainMissKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, credentialsFile)
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	restore := swapCreds(t, "darwin", func() ([]byte, error) { return nil, errors.New("no keychain") })
	defer restore()

	if got := resolveCredsFile(dir); got != path {
		t.Fatalf("path = %q, want %q", got, path)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "existing" {
		t.Fatalf("file was overwritten: %q", data)
	}
}

func TestResolveCredsFileNonDarwinNeverReadsKeychain(t *testing.T) {
	dir := t.TempDir()
	called := false
	restore := swapCreds(t, "linux", func() ([]byte, error) { called = true; return []byte("x"), nil })
	defer restore()

	got := resolveCredsFile(dir)

	if called {
		t.Fatal("keychain was read on a non-darwin platform")
	}
	if want := filepath.Join(dir, credentialsFile); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestKeychainReaderErrorsWhenSecurityUnavailable(t *testing.T) {
	// Emptying PATH makes the "security" binary unresolvable on any OS, so the
	// real reader exercises its error branch without ever touching a Keychain.
	// Its success branch (line 27) needs a real macOS Keychain and is left
	// uncovered by design — the test charter forbids real Trousseau access.
	t.Setenv("PATH", "")

	got, err := keychainReader()

	if err == nil {
		t.Fatalf("keychainReader() error = nil, want error")
	}
	if got != nil {
		t.Errorf("keychainReader() data = %q, want nil", got)
	}
}

func swapCreds(t *testing.T, goos string, reader func() ([]byte, error)) func() {
	t.Helper()
	prevOS, prevReader := runtimeGOOS, keychainReader
	runtimeGOOS, keychainReader = goos, reader
	return func() { runtimeGOOS, keychainReader = prevOS, prevReader }
}
