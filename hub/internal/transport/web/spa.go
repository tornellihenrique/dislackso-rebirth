package web

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const indexFile = "index.html"

// contentSecurityPolicy applies to the SPA document. connect-src 'self'
// also covers same-origin ws:/wss: in current browsers.
const contentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; " +
	"form-action 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; " +
	"media-src 'self' blob:; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:"

// reservedPrefixes belong to the API and never fall back to index.html, so a
// typo in an API path returns 404 instead of an HTML page.
var reservedPrefixes = []string{"/rpc/", "/api/", "/files/", "/gateway", "/lk/", "/internal/"}

type spaHandler struct {
	files fs.FS
}

// newSPAHandler serves the built web client. Unknown extension-less paths get
// index.html so client-side routes survive a reload.
func newSPAHandler(files fs.FS) http.Handler {
	return &spaHandler{files: files}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	clean := path.Clean("/" + r.URL.Path)
	for _, p := range reservedPrefixes {
		if clean == strings.TrimSuffix(p, "/") || strings.HasPrefix(clean, p) {
			http.NotFound(w, r)
			return
		}
	}

	name := strings.TrimPrefix(clean, "/")
	if name == "" || name == indexFile {
		h.serveIndex(w, r)
		return
	}
	if hasDotSegment(name) {
		http.NotFound(w, r)
		return
	}
	if info, err := fs.Stat(h.files, name); err == nil && !info.IsDir() {
		if strings.HasPrefix(name, "assets/") {
			// Vite fingerprints everything under assets/.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeFileFS(w, r, h.files, name) //nolint:gosec // fs.FS rejects names that leave its root
		return
	}
	if path.Ext(name) != "" {
		http.NotFound(w, r)
		return
	}
	h.serveIndex(w, r)
}

func (h *spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	index, err := fs.ReadFile(h.files, indexFile)
	if errors.Is(err, fs.ErrNotExist) {
		http.Error(w, "web client not built: run `task build`, or use the Vite dev server from `task dev`", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	http.ServeContent(w, r, indexFile, time.Time{}, bytes.NewReader(index))
}

func hasDotSegment(name string) bool {
	for seg := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}
