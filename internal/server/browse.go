package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

// maxViewBytes caps the size of a file streamed to the in-browser viewer.
const maxViewBytes = 512 * 1024

type browseEntry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
}

type browseResult struct {
	Path    string        `json:"path"`
	Parent  string        `json:"parent"`
	Root    string        `json:"root"`
	Entries []browseEntry `json:"entries"`
}

// handleBrowse lists a host directory for the path picker. It is intentionally
// unrestricted across the filesystem: the server is localhost-only and runs as
// the user who can already read these files (and mount them into the sandbox).
// mode=file also lists regular files; otherwise only directories are returned.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	// The picker always sends an absolute path; anything else (empty, relative,
	// an unexpanded ~) falls back to the root so browsing never leaks the
	// server's working directory.
	path := r.URL.Query().Get("path")
	if !filepath.IsAbs(path) {
		path = s.root
	}
	path = filepath.Clean(path)
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		path = s.root
	}

	ents, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res := browseResult{Path: path, Root: s.root, Entries: []browseEntry{}}
	if parent := filepath.Dir(path); parent != path {
		res.Parent = parent
	}
	for _, e := range ents {
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(path, e.Name())); err == nil {
				isDir = st.IsDir()
			}
		}
		if !isDir && mode != "file" {
			continue
		}
		res.Entries = append(res.Entries, browseEntry{Name: e.Name(), Dir: isDir})
	}
	sort.Slice(res.Entries, func(i, j int) bool {
		if res.Entries[i].Dir != res.Entries[j].Dir {
			return res.Entries[i].Dir // directories first
		}
		return res.Entries[i].Name < res.Entries[j].Name
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// handleFile streams a host file's text content to the in-browser viewer. It
// serves regular files only, caps the size and refuses binary content. Same
// posture as handleBrowse: localhost-only, read-only.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !filepath.IsAbs(path) {
		http.Error(w, "chemin absolu requis", http.StatusBadRequest)
		return
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.Error(w, "fichier introuvable", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if info.Size() > maxViewBytes {
		fmt.Fprintf(w, "… fichier trop volumineux pour l'aperçu (%d octets)\n", info.Size())
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !utf8.Valid(b) {
		io.WriteString(w, "… contenu binaire non affiché\n")
		return
	}
	_, _ = w.Write(b)
}
