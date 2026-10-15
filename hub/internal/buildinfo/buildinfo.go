// Package buildinfo reports which build of the hub is running.
package buildinfo

import "runtime/debug"

// version is set at link time with -ldflags "-X <this package>.version=<v>".
var version string

// Version returns the link-time version, falling back to the module version
// stamped by the Go toolchain, then to "dev".
func Version() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
