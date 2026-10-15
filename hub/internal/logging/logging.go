// Package logging builds the hub's slog logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"
)

// New returns a logger writing to w. format is "text" or "json".
func New(w io.Writer, level, format string) (*slog.Logger, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	opts := &slog.HandlerOptions{Level: lvl}
	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("unknown log format %q", format)
	}
}
