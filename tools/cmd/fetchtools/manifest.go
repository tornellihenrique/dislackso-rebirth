package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type archiveKind string

const (
	archiveRaw      archiveKind = "raw"
	archiveZip      archiveKind = "zip"
	archiveTarGz    archiveKind = "tar.gz"
	archiveGoSource archiveKind = "go-source"
)

type manifest struct {
	Tools []tool `json:"tools"`
}

type tool struct {
	Name      string              `json:"name"`
	Version   string              `json:"version"`
	Platforms map[string]artifact `json:"platforms"`
	// Source is built with the local Go toolchain on platforms without a published binary.
	Source *artifact `json:"source,omitempty"`
}

type artifact struct {
	URL     string      `json:"url"`
	SHA256  string      `json:"sha256"`
	Archive archiveKind `json:"archive"`
	// Path is the binary inside an archive, or the module root inside a source archive.
	Path string `json:"path,omitempty"`
	// Package is the main package to build, relative to Path, for go-source archives.
	Package string `json:"package,omitempty"`
}

func loadManifest(path string) (manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return m, nil
}

func (m manifest) selectTools(names []string) ([]tool, error) {
	if len(names) == 0 {
		return m.Tools, nil
	}
	byName := make(map[string]tool, len(m.Tools))
	for _, t := range m.Tools {
		byName[t.Name] = t
	}
	selected := make([]tool, 0, len(names))
	for _, n := range names {
		t, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("unknown tool %q", n)
		}
		selected = append(selected, t)
	}
	return selected, nil
}

// artifactFor returns the artifact to install on platform ("goos/goarch") with
// the {version} placeholder expanded.
func (t tool) artifactFor(platform string) (artifact, error) {
	a, ok := t.Platforms[platform]
	if !ok {
		if t.Source == nil {
			return artifact{}, fmt.Errorf("no build for %s", platform)
		}
		a = *t.Source
	}
	expand := strings.NewReplacer("{version}", t.Version)
	a.URL = expand.Replace(a.URL)
	a.Path = expand.Replace(a.Path)
	return a, nil
}
