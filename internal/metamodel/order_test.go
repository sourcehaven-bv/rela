package metamodel

import (
	"math"
	"testing"
)

func TestCompareOrderKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b OrderKey
		want int
	}{
		{"lower value first", OrderKey{Value: 1.0, Peer: "B"}, OrderKey{Value: 2.0, Peer: "A"}, -1},
		{"integer and float values compare as numbers", OrderKey{Value: 3, Peer: "A"}, OrderKey{Value: 2.5, Peer: "B"}, 1},
		{"a value before none", OrderKey{Value: 9.0, Peer: "Z"}, OrderKey{Peer: "A"}, -1},
		{"none after a value", OrderKey{Peer: "A"}, OrderKey{Value: 9.0, Peer: "Z"}, 1},
		{"a non-finite value counts as none", OrderKey{Value: math.NaN(), Peer: "A"}, OrderKey{Value: 1.0, Peer: "Z"}, 1},
		{"equal values fall back to the peer", OrderKey{Value: 1.0, Peer: "A"}, OrderKey{Value: 1.0, Peer: "B"}, -1},
		{"two without a value fall back to the peer", OrderKey{Peer: "B"}, OrderKey{Peer: "A"}, 1},
		{"one peer falls back to the tail", OrderKey{Value: 1.0, Peer: "A", Tail: "draft"}, OrderKey{Value: 1.0, Peer: "A", Tail: "published"}, -1},
		{"identical keys are equal", OrderKey{Value: 1.0, Peer: "A", Tail: "t"}, OrderKey{Value: 1.0, Peer: "A", Tail: "t"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CompareOrderKeys(tt.a, tt.b); got != tt.want {
				t.Errorf("CompareOrderKeys(%+v, %+v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
