package server

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/keyxmare/claude-benchy/internal/runner"
)

// handleApply applies a run's captured diff onto the project the benchmark ran
// against, so a config's output can be adopted in one click. It runs
// `git apply` in the app's working tree and leaves the changes unstaged for
// review — it never commits. Localhost-only, like the rest of the dashboard.
func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	dirParam := r.FormValue("dir")
	dir, err := s.safeDir(dirParam)
	if err != nil {
		writeApplyPage(w, http.StatusBadRequest, dirParam, err.Error(), false)
		return
	}
	patch, err := resolvePatch(dir, r.FormValue("artifact"))
	if err != nil {
		writeApplyPage(w, http.StatusBadRequest, dirParam, err.Error(), false)
		return
	}
	_, app := runner.BenchInfo(dir)
	if strings.TrimSpace(app) == "" {
		writeApplyPage(w, http.StatusBadRequest, dirParam, "chemin de l'app introuvable dans le bench", false)
		return
	}
	if out, err := gitApply(app, patch); err != nil {
		writeApplyPage(w, http.StatusUnprocessableEntity, dirParam, "git apply a échoué : "+out, false)
		return
	}
	writeApplyPage(w, http.StatusOK, dirParam, "Modifications appliquées dans "+app+" (non committées).", true)
}

// resolvePatch returns the diff.patch path for a run's artifact directory,
// rejecting an artifact path that escapes the results directory.
func resolvePatch(resultsDir, artifact string) (string, error) {
	if strings.TrimSpace(artifact) == "" {
		return "", errors.New("artefact manquant")
	}
	art := filepath.Clean(filepath.Join(resultsDir, artifact))
	rel, err := filepath.Rel(resultsDir, art)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("artefact hors du dossier de résultats")
	}
	patch := filepath.Join(art, "diff.patch")
	if _, err := os.Stat(patch); err != nil {
		return "", errors.New("diff introuvable pour cette config")
	}
	return patch, nil
}

// gitApply applies patch to the working tree of the git repository at app,
// leaving the changes unstaged. safe.directory=* keeps git from refusing a tree
// owned by another uid (the dashboard container runs as a fixed user). The
// combined output is returned flattened to one line for display.
func gitApply(app, patch string) (string, error) {
	cmd := exec.Command("git", "-c", "safe.directory=*", "-C", app, "apply", patch)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.ReplaceAll(strings.TrimSpace(out.String()), "\n", " / "), err
}

// writeApplyPage renders the outcome of an apply with a link back to the report.
func writeApplyPage(w http.ResponseWriter, code int, backDir, message string, ok bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	title := "Échec de l'application"
	if ok {
		title = "Modifications appliquées"
	}
	back := "/report?dir=" + url.QueryEscape(backDir)
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="fr"><head><meta charset="utf-8">`+
		`<title>%s</title></head><body style="font-family:system-ui;max-width:44rem;margin:3rem auto;padding:0 1rem">`+
		`<h1>%s</h1><p>%s</p><p><a href="%s">← Retour au rapport</a></p></body></html>`,
		html.EscapeString(title), html.EscapeString(title), html.EscapeString(message), back)
}
