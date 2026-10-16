//go:build !windows

package main

import "io"

func holdWindow(io.Writer) {}
