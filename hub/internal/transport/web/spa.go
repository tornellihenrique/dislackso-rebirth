package web

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"
)

const indexFile = "index.html"

// contentSecurityPolicy applies to the SPA document. connect-src 'self'
// also covers same-origin ws:/wss: in current browsers. Styles allow a
// per-response nonce: UI libraries inject <style> elements (scroll locking
// in modal dialogs and menus) and tag them with the nonce the page hands them.
func contentSecurityPolicy(nonce string) string {
	return "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; " +
		"form-action 'self'; script-src 'self'; style-src 'self' 'nonce-" + nonce + "'; img-src 'self' data: blob:; " +
		"media-src 'self' blob:; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:"
}

// nonceMeta carries the style nonce to the web client, which reads it at startup.
const nonceMeta = "csp-nonce"

func newNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

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
	nonce, err := newNonce()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	meta := []byte(`<meta name="` + nonceMeta + `" content="` + nonce + `">`)
	if i := bytes.Index(index, []byte("<head>")); i >= 0 {
		index = slices.Concat(index[:i+len("<head>")], meta, index[i+len("<head>"):])
	} else {
		index = append(meta, index...)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// A nonce must never be reused, so the page is never cached.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy(nonce))
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
