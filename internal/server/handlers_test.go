package server_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keyxmare/claude-benchy/internal/docker"
)

// blockingDocker keeps the Claude run parked on release so a benchmark stays in
// the running state long enough to observe stop and client-disconnect handling.
type blockingDocker struct{ release chan struct{} }

func (blockingDocker) Build(context.Context, string, string, string, map[string]string) error {
	return nil
}

func (b blockingDocker) Run(_ context.Context, spec docker.RunSpec, stdout, _ io.Writer) error {
	if spec.Entrypoint != "" {
		return nil
	}
	<-b.release
	_ = os.WriteFile(filepath.Join(spec.WorkDir, "added.txt"), []byte("edited\n"), 0o644)
	_, _ = fmt.Fprintln(stdout, `{"type":"result","subtype":"success","is_error":false,"num_turns":1,"total_cost_usd":0.01,"result":"ok"}`)
	return nil
}

// removeOutputDocker wipes the run's output tree mid-run so report.Write cannot
// create its files, driving the execution to the failed state.
type removeOutputDocker struct{}

func (removeOutputDocker) Build(context.Context, string, string, string, map[string]string) error {
	return nil
}

func (removeOutputDocker) Run(_ context.Context, spec docker.RunSpec, _, _ io.Writer) error {
	if spec.Entrypoint != "" {
		return nil
	}
	outputRoot := filepath.Dir(filepath.Dir(spec.WorkDir))
	_ = os.RemoveAll(outputRoot)
	return nil
}

// failWriter is an http.ResponseWriter whose body writes always fail, so a
// handler's template/report render error path is exercised.
type failWriter struct{ h http.Header }

func (f *failWriter) Header() http.Header {
	if f.h == nil {
		f.h = http.Header{}
	}
	return f.h
}
func (f *failWriter) Write([]byte) (int, error) { return 0, errors.New("boom") }
func (f *failWriter) WriteHeader(int)           {}

// nonFlusherWriter is an http.ResponseWriter that is deliberately not an
// http.Flusher, so the SSE handler's streaming-unsupported guard is reached.
type nonFlusherWriter struct {
	header http.Header
	code   int
}

func (n *nonFlusherWriter) Header() http.Header {
	if n.header == nil {
		n.header = http.Header{}
	}
	return n.header
}
func (n *nonFlusherWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n *nonFlusherWriter) WriteHeader(code int)        { n.code = code }

// runToCompletion posts the standard bench and blocks on the event stream until
// the run finishes, returning the produced run directory relative to root.
func runToCompletion(t *testing.T, h http.Handler, root string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(runForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("run status = %d, want 303; body: %s", rec.Code, rec.Body.String())
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/runs/1/events", nil))
	return latestRunDir(t, root)
}

func TestRunPageRendersAndNotFound(t *testing.T) {
	srv, root := newTestServer(t)
	h := srv.Handler()
	runToCompletion(t, h, root)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/runs/1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("run page status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "done") {
		t.Errorf("run page should show the finished status; body:\n%s", rec.Body.String())
	}

	nf := httptest.NewRecorder()
	h.ServeHTTP(nf, httptest.NewRequest(http.MethodGet, "/runs/999", nil))
	if nf.Code != http.StatusNotFound {
		t.Errorf("unknown run: status = %d, want 404", nf.Code)
	}
}

func TestStopCancelsRunAndNotFound(t *testing.T) {
	release := make(chan struct{})
	srv, _ := serverWith(t, blockingDocker{release: release})
	h := srv.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(runForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("run status = %d, want 303", rec.Code)
	}

	nf := httptest.NewRecorder()
	h.ServeHTTP(nf, httptest.NewRequest(http.MethodPost, "/runs/999/stop", nil))
	if nf.Code != http.StatusNotFound {
		t.Errorf("stopping an unknown run: status = %d, want 404", nf.Code)
	}

	stopRec := httptest.NewRecorder()
	h.ServeHTTP(stopRec, httptest.NewRequest(http.MethodPost, "/runs/1/stop", nil))
	if stopRec.Code != http.StatusSeeOther || stopRec.Header().Get("Location") != "/runs/1" {
		t.Fatalf("stop = %d %q, want 303 /runs/1", stopRec.Code, stopRec.Header().Get("Location"))
	}

	close(release)
	evRec := httptest.NewRecorder()
	h.ServeHTTP(evRec, httptest.NewRequest(http.MethodGet, "/runs/1/events", nil))
	if !strings.Contains(evRec.Body.String(), "data: stopped") {
		t.Errorf("a cancelled run should end stopped; stream:\n%s", evRec.Body.String())
	}
}

func TestEventsUnknownRunNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/runs/999/events", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestEventsRequireFlusher(t *testing.T) {
	srv, root := newTestServer(t)
	h := srv.Handler()
	runToCompletion(t, h, root)

	w := &nonFlusherWriter{}
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/runs/1/events", nil))
	if w.code != http.StatusInternalServerError {
		t.Fatalf("streaming guard: status = %d, want 500", w.code)
	}
}

func TestEventsStopWhenClientDisconnects(t *testing.T) {
	release := make(chan struct{})
	srv, _ := serverWith(t, blockingDocker{release: release})
	h := srv.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(runForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the client is already gone before the stream loop parks
	evReq := httptest.NewRequest(http.MethodGet, "/runs/1/events", nil).WithContext(ctx)
	evRec := httptest.NewRecorder()
	h.ServeHTTP(evRec, evReq) // returns via ctx.Done rather than blocking on the run
	if strings.Contains(evRec.Body.String(), "data: done") {
		t.Errorf("a disconnected stream should not emit a terminal event; stream:\n%s", evRec.Body.String())
	}

	close(release)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/runs/1/events", nil))
}

func TestEventsReportFailureCarriesMessage(t *testing.T) {
	srv, _ := serverWith(t, removeOutputDocker{})
	h := srv.Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(runForm().Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)

	evRec := httptest.NewRecorder()
	h.ServeHTTP(evRec, httptest.NewRequest(http.MethodGet, "/runs/1/events", nil))
	stream := evRec.Body.String()
	if !strings.Contains(stream, "event: done\ndata: failed:") {
		t.Errorf("a failed run should end with a failed message; stream:\n%s", stream)
	}
}

func TestRunSpecErrorRerendersForm(t *testing.T) {
	srv, _ := newTestServer(t)
	form := runForm()
	form.Set("runs", "abc") // parsed before the spec is built

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRunReserveDirErrorIsServerError(t *testing.T) {
	srv, root := newTestServer(t)
	if err := os.WriteFile(filepath.Join(root, "notadir"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	form := runForm()
	form.Set("output", "./notadir") // a regular file cannot host the timestamped dir

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestReportRejectsDirWithoutBench(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	// "app" is inside the root but is not a benchmark output directory.
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/report?dir=app", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestTranscriptMissingDirParam(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/transcript?dir=", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestTranscriptMissingFileIsNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	// "app" resolves within the root but holds no transcript.jsonl.
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/transcript?dir=app", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestRenderErrorsAreSurvived drives every handler that renders through a
// template or the report writer with a writer whose body writes fail, covering
// their render-error branches (which only log to stderr).
func TestRenderErrorsAreSurvived(t *testing.T) {
	srv, root := newTestServer(t)
	h := srv.Handler()
	dir := runToCompletion(t, h, root)

	cases := []struct {
		name   string
		method string
		target string
	}{
		{"dashboard", http.MethodGet, "/"},
		{"run page", http.MethodGet, "/runs/1"},
		{"report", http.MethodGet, "/report?dir=" + url.QueryEscape(dir)},
		{"transcript", http.MethodGet, "/transcript?dir=" + url.QueryEscape(dir+"/baseline")},
		{"apply page", http.MethodPost, "/apply?dir=../../etc"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(*testing.T) {
			// No assertion: the handler swallows the write error, so reaching the
			// call without panicking is the coverage under test.
			h.ServeHTTP(&failWriter{}, httptest.NewRequest(tc.method, tc.target, nil))
		})
	}
}

func TestBrowseFallsBackToRoot(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.Handler()

	tests := map[string]string{
		"relative path": "/browse",                        // empty path is not absolute
		"missing path":  "/browse?path=/no/such/absolute", // stat fails
	}
	for name, target := range tests {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), `"app"`) {
				t.Errorf("browse should have fallen back to the root; body:\n%s", rec.Body.String())
			}
		})
	}
}

func TestBrowseSymlinksAndModes(t *testing.T) {
	srv, root := newTestServer(t)
	h := srv.Handler()

	dir := filepath.Join(root, "browsedir")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "symdir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "file.txt"), filepath.Join(dir, "symfile")); err != nil {
		t.Fatal(err)
	}

	fileRec := httptest.NewRecorder()
	h.ServeHTTP(fileRec, httptest.NewRequest(http.MethodGet, "/browse?mode=file&path="+url.QueryEscape(dir), nil))
	fileBody := fileRec.Body.String()
	for _, want := range []string{`"sub"`, `"file.txt"`, `"symdir"`, `"symfile"`} {
		if !strings.Contains(fileBody, want) {
			t.Errorf("file mode should list %s; body:\n%s", want, fileBody)
		}
	}

	dirRec := httptest.NewRecorder()
	h.ServeHTTP(dirRec, httptest.NewRequest(http.MethodGet, "/browse?mode=dir&path="+url.QueryEscape(dir), nil))
	dirBody := dirRec.Body.String()
	if !strings.Contains(dirBody, `"sub"`) || !strings.Contains(dirBody, `"symdir"`) {
		t.Errorf("dir mode should list directories and symlinked dirs; body:\n%s", dirBody)
	}
	if strings.Contains(dirBody, `"file.txt"`) || strings.Contains(dirBody, `"symfile"`) {
		t.Errorf("dir mode should skip regular files and symlinked files; body:\n%s", dirBody)
	}
}

func TestFileRejectsRelativePath(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/file?path=relative.txt", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestFileTooLargeIsSummarised(t *testing.T) {
	srv, root := newTestServer(t)
	big := filepath.Join(root, "big.txt")
	if err := os.WriteFile(big, make([]byte, 600*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/file?path="+url.QueryEscape(big), nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "trop volumineux") {
		t.Fatalf("status = %d, body = %q, want a too-large notice", rec.Code, rec.Body.String())
	}
}

func TestFileBinaryIsNotShown(t *testing.T) {
	srv, root := newTestServer(t)
	bin := filepath.Join(root, "bin.dat")
	if err := os.WriteFile(bin, []byte{0xff, 0xfe, 0x00, 0x01}, 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/file?path="+url.QueryEscape(bin), nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "binaire") {
		t.Fatalf("status = %d, body = %q, want a binary notice", rec.Code, rec.Body.String())
	}
}

// The following production branches are intentionally not exercised: they are
// unreachable when the test suite runs as root in the toolchain container, or
// never fail for a well-formed input.
//   - server.New: filepath.Abs only errors when the working directory is
//     unavailable, which cannot happen in a test.
//   - handleRun: yaml.Marshal of the persisted bench document never fails (no
//     unsupported types).
//   - handleReport: runner.Reload only errors on a filesystem walk error, which
//     root never encounters over a valid output tree.
//   - execution.exec: runner.Run only errors if MkdirAll of the already-created
//     output root fails, which cannot happen.
//   - reserveDir: os.Mkdir under a writable output directory returns either
//     success or fs.ErrExist, never another error.
//   - handleBrowse / handleFile: os.ReadDir / os.ReadFile after a successful
//     stat only fail on a permission error, which root bypasses.
//   - scanHistory: filepath.Rel of two absolute paths under the same root never
//     errors on the test platform.
var _ = struct{}{}
