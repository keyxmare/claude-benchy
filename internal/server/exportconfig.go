package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/keyxmare/claude-benchy/internal/runner"
)

// runBundle returns the config bundle directory recorded for a run's artifact
// (in meta.json), validated to exist. Empty when the run predates bundle
// recording — the user must re-run the bench to export its config.
func runBundle(resultsDir, artifact string) (string, error) {
	art, err := artifactDir(resultsDir, artifact)
	if err != nil {
		return "", err
	}
	var meta struct {
		Bundle string `json:"bundle"`
	}
	if b, err := os.ReadFile(filepath.Join(art, "meta.json")); err == nil {
		_ = json.Unmarshal(b, &meta)
	}
	if strings.TrimSpace(meta.Bundle) == "" {
		return "", errors.New("bundle inconnu pour ce run (relancer le bench pour exporter la conf)")
	}
	if info, err := os.Stat(meta.Bundle); err != nil || !info.IsDir() {
		return "", errors.New("dossier de bundle introuvable")
	}
	return meta.Bundle, nil
}

// listBundleFiles returns the bundle's files as slash paths relative to it. The
// bundle is already validated to exist, so unreadable entries are skipped.
func listBundleFiles(bundle string) []string {
	var files []string
	_ = filepath.WalkDir(bundle, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if rel, e := filepath.Rel(bundle, p); e == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files
}

// handleConfigFiles lists a run's config bundle files for the export picker.
func (s *Server) handleConfigFiles(w http.ResponseWriter, r *http.Request) {
	dir, err := s.safeDir(r.URL.Query().Get("dir"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	bundle, err := runBundle(dir, r.URL.Query().Get("artifact"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"files": listBundleFiles(bundle)})
}

// handleExportConfig copies the selected config bundle files onto the project
// under test, overwriting existing ones, left unstaged for review.
func (s *Server) handleExportConfig(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	dirParam := r.FormValue("dir")
	dir, err := s.safeDir(dirParam)
	if err != nil {
		writeApplyPage(w, http.StatusBadRequest, dirParam, err.Error(), false)
		return
	}
	bundle, err := runBundle(dir, r.FormValue("artifact"))
	if err != nil {
		writeApplyPage(w, http.StatusBadRequest, dirParam, err.Error(), false)
		return
	}
	_, app := runner.BenchInfo(dir)
	if strings.TrimSpace(app) == "" {
		writeApplyPage(w, http.StatusBadRequest, dirParam, "chemin de l'app introuvable dans le bench", false)
		return
	}
	files := r.Form["files"]
	if len(files) == 0 {
		writeApplyPage(w, http.StatusBadRequest, dirParam, "aucun fichier sélectionné", false)
		return
	}
	n, err := exportFiles(bundle, app, files)
	if err != nil {
		writeApplyPage(w, http.StatusUnprocessableEntity, dirParam, "export interrompu : "+err.Error(), false)
		return
	}
	writeApplyPage(w, http.StatusOK, dirParam, plural(n)+" dans "+app+" (non committé).", true)
}

// exportFiles copies each selected bundle file to the app at the same relative
// path, creating parent directories and overwriting existing files. A selection
// that is absolute or escapes via ".." is rejected before any copy.
func exportFiles(bundle, app string, rels []string) (int, error) {
	copied := 0
	for _, rel := range rels {
		if rel == "" || strings.HasPrefix(rel, "/") || hasDotDot(rel) {
			return copied, errors.New("chemin de fichier invalide : " + rel)
		}
		clean := filepath.FromSlash(rel)
		if err := copyFile(filepath.Join(bundle, clean), filepath.Join(app, clean)); err != nil {
			return copied, err
		}
		copied++
	}
	return copied, nil
}

// hasDotDot reports whether a slash path has a ".." segment.
func hasDotDot(rel string) bool {
	for _, p := range strings.Split(rel, "/") {
		if p == ".." {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func plural(n int) string {
	if n == 1 {
		return "1 fichier de conf exporté"
	}
	return strconv.Itoa(n) + " fichiers de conf exportés"
}
