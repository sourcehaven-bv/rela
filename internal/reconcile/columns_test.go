package reconcile_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/reconcile"
)

// TestRowColumns pins that the stored columns of a row are the element of
// that row, and that summing stored limbs folds back to the fingerprint sum.
func TestRowColumns(t *testing.T) {
	e := ticket()
	e.Face = "draft"
	r := edge()

	ec := reconcile.EntityRowColumns(e.Ref(), e.Type, e.Properties, e.Content)
	rc := reconcile.RelationRowColumns(r.Identity(), r.Properties, r.Content)
	eEl, rEl := reconcile.EntityElement(e), reconcile.RelationElement(r)

	for _, tc := range []struct {
		name string
		got  reconcile.Columns
		el   reconcile.Element
	}{{"entity", ec, eEl}, {"relation", rc, rEl}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.el.Columns() {
				t.Fatalf("row columns %+v, element columns %+v", tc.got, tc.el.Columns())
			}
			if tc.got.Hash != tc.el.Hash.String() {
				t.Fatalf("hash %q, want %q", tc.got.Hash, tc.el.Hash.String())
			}
			args := tc.got.Args()
			if len(args) != 5 || args[0] != tc.got.Hash || args[4] != tc.got.Limbs[3] {
				t.Fatalf("args %v not in column order", args)
			}
		})
	}

	var sums [4]uint64
	for _, c := range []reconcile.Columns{ec, rc} {
		for i, l := range c.Limbs {
			if l < 0 || l >= 1<<32 {
				t.Fatalf("limb %d = %d is not a 32-bit value", i, l)
			}
			sums[i] += uint64(l)
		}
	}
	want := eEl.Fingerprint().Add(rEl.Fingerprint())
	if got := reconcile.FingerprintFromLimbSums(sums); got != want {
		t.Fatalf("folded sum %+v, want %+v", got, want)
	}
}
