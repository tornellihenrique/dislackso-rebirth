// Package ports declares the interfaces services depend on. Adapters
// implement them and the composition root wires the concrete types.
package ports

import "time"

// Clock is the source of the current time.
type Clock interface {
	Now() time.Time
}
