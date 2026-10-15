// Package webdist embeds the production web client. `task build` writes the
// Vite output into dist/; the committed .gitkeep keeps the embed pattern
// valid when the client has not been built.
package webdist

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the build rooted at index.html.
func FS() (fs.FS, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded web build: %w", err)
	}
	return sub, nil
}
