package app

import "testing"

func TestAddressWatch(t *testing.T) {
	const old, next, other = "203.0.113.1", "203.0.113.2", "203.0.113.3"
	tests := []struct {
		name       string
		advertised string
		seen       []string
		want       []bool
	}{
		{"same address", old, []string{old, old}, []bool{false, false}},
		{"a new address, seen twice", old, []string{next, next}, []bool{false, true}},
		{"one odd answer", old, []string{next, old, next}, []bool{false, false, false}},
		{"two different answers", old, []string{next, other, other}, []bool{false, false, true}},
		{"offline says nothing", old, []string{"", "", next, "", next}, []bool{false, false, false, false, false}},
		{"nothing advertised cannot go stale", "", []string{next, next, next}, []bool{false, false, false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w addressWatch
			for i, seen := range tt.seen {
				if got := w.moved(tt.advertised, seen); got != tt.want[i] {
					t.Errorf("look %d at %q = %v, want %v", i, seen, got, tt.want[i])
				}
			}
		})
	}
}
