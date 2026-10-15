// Package clock implements ports.Clock.
package clock

import "time"

// System reads the operating system clock.
type System struct{}

func (System) Now() time.Time { return time.Now() }
