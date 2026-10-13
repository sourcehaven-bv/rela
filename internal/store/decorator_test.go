package store_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// backend is a store that wraps nothing.
type backend struct{ store.Store }

// decorated is a store that wraps another.
type decorated struct{ store.Store }

func (d decorated) Unwrap() store.Store { return d.Store }

func TestInnermost(t *testing.T) {
	base := &backend{}
	for _, tc := range []struct {
		name string
		st   store.Store
		want store.Store
	}{
		{name: "undecorated", st: base, want: base},
		{name: "one layer", st: decorated{base}, want: base},
		{name: "two layers", st: decorated{decorated{base}}, want: base},
		{name: "nil", st: nil, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := store.Innermost(tc.st); got != tc.want {
				t.Errorf("Innermost = %#v, want %#v", got, tc.want)
			}
		})
	}
}
