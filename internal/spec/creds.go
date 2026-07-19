package spec

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// keychainService is the macOS Keychain generic-password service under which
// Claude Code stores its OAuth credentials. On macOS the credentials live in
// the Keychain rather than in configDir/.credentials.json, so mounting that
// file directly would fail — they are exported from the Keychain first.
const keychainService = "Claude Code-credentials"

// runtimeGOOS and keychainReader are indirected through variables so the
// credentials resolution can be exercised on any platform in tests.
var (
	runtimeGOOS = runtime.GOOS

	keychainReader = func() ([]byte, error) {
		out, err := exec.Command("security", "find-generic-password", "-s", keychainService, "-w").Output()
		if err != nil {
			return nil, err
		}
		return bytes.TrimRight(out, "\r\n"), nil
	}
)

// resolveCredsFile returns the credentials file to mount into the sandbox.
//
// On macOS it exports the Keychain-held credentials into configDir so the file
// always reflects the current token, falling back to any existing file when the
// Keychain cannot be read. On other platforms it uses configDir/.credentials.json
// as-is (the file the host already maintains).
func resolveCredsFile(configDir string) string {
	path := filepath.Join(configDir, credentialsFile)
	if runtimeGOOS == "darwin" {
		if data, err := keychainReader(); err == nil && len(data) > 0 {
			if os.MkdirAll(configDir, 0o755) == nil && os.WriteFile(path, data, 0o600) == nil {
				return path
			}
		}
	}
	return path
}
