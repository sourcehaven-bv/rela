package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestCondConstant(t *testing.T) {
	leaf := store.PropCond(store.PropPredicate{Property: "s", Op: store.PropEqual, Value: "x"})
	f, tr := store.OrCond(), store.AndCond()
	for _, tc := range []struct {
		name         string
		c            store.Cond
		value, known bool
	}{
		{"leaf", leaf, false, false},
		{"empty or", f, false, true},
		{"empty and", tr, true, true},
		{"not false", store.NotCond(f), true, true},
		{"and with false", store.AndCond(leaf, f), false, true},
		{"and with true", store.AndCond(leaf, tr), false, false},
		{"or with true", store.OrCond(leaf, store.NotCond(f)), true, true},
		{"or of falses", store.OrCond(f, store.AndCond(f, leaf)), false, true},
		{"not leaf", store.NotCond(leaf), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, k := tc.c.Constant()
			require.Equal(t, tc.known, k)
			if k {
				require.Equal(t, tc.value, v)
			}
		})
	}
}
