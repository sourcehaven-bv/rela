package reconcile_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
)

func ticket() entity.Entity {
	return entity.Entity{
		ID:         "TKT-1",
		Type:       "ticket",
		Properties: map[string]any{"title": "Hello", "priority": 3},
		Content:    "Body text.",
	}
}

func edge() entity.Relation {
	return entity.Relation{
		From:       "TKT-1",
		Type:       "affects",
		To:         "CON-1",
		Properties: map[string]any{"weight": 2},
	}
}

func hex(f reconcile.Fingerprint) string { return fmt.Sprintf("%016x%016x", f.Hi, f.Lo) }

// The fingerprint encoding is a wire and storage format: stored fingerprint
// columns and a peer's fingerprints must keep matching. A change here needs
// a new domain tag and a migration, not a new golden.
func TestFingerprint_Golden(t *testing.T) {
	faced := ticket()
	faced.Face = "draft"
	tailed := edge()
	tailed.FromFace = "draft"

	cases := []struct {
		name    string
		el      reconcile.Element
		address string
		fp      string
	}{
		{"entity", reconcile.EntityElement(ticket()), "ticket/TKT-1", "9265845eb2662ed7c12ccf91aec65aae"},
		{"faced entity", reconcile.EntityElement(faced), "ticket/TKT-1@draft", "6d35514c03a472558d01d9e84138dcfb"},
		{"relation", reconcile.RelationElement(edge()), "TKT-1--affects--CON-1", "555be2a1a7c0ba61a525c7badc085ca9"},
		{
			"relation with tail face", reconcile.RelationElement(tailed),
			"TKT-1@draft--affects--CON-1", "5a9c59f8202fa25622c38c605ae5f6ac",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.el.Address != tc.address {
				t.Errorf("address = %q, want %q", tc.el.Address, tc.address)
			}
			if got := hex(tc.el.Fingerprint()); got != tc.fp {
				t.Errorf("fingerprint = %s, want %s", got, tc.fp)
			}
		})
	}
}

// For a reader with nothing hidden the view hash is the stored content hash,
// so a backend can fingerprint from the column it already has.
func TestElement_HashIsContentHash(t *testing.T) {
	if got, want := reconcile.EntityElement(ticket()).Hash.String(), canonical.HashEntity(ticket()); got != want {
		t.Errorf("entity hash = %s, want content hash %s", got, want)
	}
	if got, want := reconcile.RelationElement(edge()).Hash.String(), canonical.HashRelation(edge()); got != want {
		t.Errorf("relation hash = %s, want content hash %s", got, want)
	}
}

// A redacted field does not change the view hash. The redacted view (value
// removed, name in Redacted) hashes like a row where the field was never
// set, so neither the value nor whether it is set shows. The per-reader
// Inaccessible list does not count either. That any hidden value gives the
// same view is the view builder's job, pinned by the dataentry ETag tests.
func TestElement_RedactedFieldDoesNotChangeViewHash(t *testing.T) {
	unset := ticket()

	redacted := ticket()
	redacted.Redacted = []string{"salary"}

	locked := ticket()
	locked.Inaccessible = []entity.InaccessibleField{{Name: "secret", Reason: "encrypted"}}

	want := reconcile.EntityElement(unset)
	for _, view := range []entity.Entity{redacted, locked} {
		if got := reconcile.EntityElement(view); got != want {
			t.Errorf("view %+v: element %+v, want %+v", view, got, want)
		}
	}

	visible := ticket()
	visible.Properties["title"] = "Changed"
	if reconcile.EntityElement(visible).Hash == want.Hash {
		t.Error("a visible property change did not change the view hash")
	}
	body := ticket()
	body.Content = "Other body."
	if reconcile.EntityElement(body).Hash == want.Hash {
		t.Error("a body change did not change the view hash")
	}
}

func TestFingerprint_KindSeparates(t *testing.T) {
	e := reconcile.EntityElement(ticket())
	r := e
	r.Kind = reconcile.KindRelation
	if e.Fingerprint() == r.Fingerprint() {
		t.Error("same address and hash under two kinds share a fingerprint")
	}
}

func TestParseKind(t *testing.T) {
	for _, b := range []byte{'E', 'R'} {
		if k, err := reconcile.ParseKind(b); err != nil || byte(k) != b {
			t.Errorf("ParseKind(%q) = %q, %v", b, k, err)
		}
	}
	for _, b := range []byte{0, 'e', 'X'} {
		if _, err := reconcile.ParseKind(b); err == nil {
			t.Errorf("ParseKind(%q) accepted an invalid kind", b)
		}
	}
}

func TestFingerprint_InvalidKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Fingerprint of the zero Kind did not panic")
		}
	}()
	_ = reconcile.Element{Address: "ticket/TKT-1"}.Fingerprint()
}

func TestFingerprint_Arithmetic(t *testing.T) {
	maxFP := reconcile.Fingerprint{Hi: ^uint64(0), Lo: ^uint64(0)}
	one := reconcile.Fingerprint{Lo: 1}
	cases := []struct {
		name string
		got  reconcile.Fingerprint
		want reconcile.Fingerprint
	}{
		{"carry from Lo into Hi", reconcile.Fingerprint{Lo: ^uint64(0)}.Add(one), reconcile.Fingerprint{Hi: 1}},
		{"wraps at 2^128", maxFP.Add(one), reconcile.Fingerprint{}},
		{"borrow from Hi", reconcile.Fingerprint{Hi: 1}.Sub(one), reconcile.Fingerprint{Lo: ^uint64(0)}},
		{"wraps below zero", reconcile.Fingerprint{}.Sub(one), maxFP},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %s, want %s", hex(tc.got), hex(tc.want))
			}
		})
	}
}

func TestLimbs_RoundTrip(t *testing.T) {
	f := reconcile.EntityElement(ticket()).Fingerprint()
	l := f.Limbs()
	if got := reconcile.FingerprintFromLimbSums([4]uint64{
		uint64(l[0]), uint64(l[1]), uint64(l[2]), uint64(l[3]),
	}); got != f {
		t.Errorf("round trip = %s, want %s", hex(got), hex(f))
	}
	if uint64(l[0]) != f.Hi>>32 || uint64(l[3]) != f.Lo&0xffffffff {
		t.Errorf("limbs %x are not most significant first for %s", l, hex(f))
	}
}

// Summing limbs column by column, as SQL will, must give the same range sum
// as adding fingerprints, even when the column sums overflow 32 bits.
func TestFingerprintFromLimbSums_MatchesAdd(t *testing.T) {
	var want reconcile.Fingerprint
	var sums [4]uint64
	maxFP := reconcile.Fingerprint{Hi: ^uint64(0), Lo: ^uint64(0)}
	fps := []reconcile.Fingerprint{maxFP, maxFP, maxFP, {Hi: 1 << 63, Lo: 1 << 31}}
	for i := range 50 {
		e := ticket()
		e.ID = fmt.Sprintf("TKT-%d", i)
		fps = append(fps, reconcile.EntityElement(e).Fingerprint())
	}
	for _, f := range fps {
		want = want.Add(f)
		for i, l := range f.Limbs() {
			sums[i] += uint64(l)
		}
	}
	if sums[3] <= 1<<32 {
		t.Fatalf("fixture: limb sums %x do not overflow 32 bits", sums)
	}
	if got := reconcile.FingerprintFromLimbSums(sums); got != want {
		t.Errorf("from limb sums = %s, want %s", hex(got), hex(want))
	}

	// Golden: three copies of 2^128-1 plus 2^127+2^31 is 2^127+2^31-3
	// modulo 2^128.
	got := reconcile.FingerprintFromLimbSums([4]uint64{
		3*0xffffffff + 0x80000000, 3 * 0xffffffff, 3 * 0xffffffff, 3*0xffffffff + 0x80000000,
	})
	if want := (reconcile.Fingerprint{Hi: 1 << 63, Lo: 1<<31 - 3}); got != want {
		t.Errorf("golden limb fold = %s, want %s", hex(got), hex(want))
	}
}

func TestRange(t *testing.T) {
	a := reconcile.EntityElement(ticket())
	b := reconcile.RelationElement(edge())
	c := ticket()
	c.ID = "TKT-2"
	ce := reconcile.EntityElement(c)

	ab := reconcile.Range{}.Add(a).Add(b)
	ba := reconcile.Range{}.Add(b).Add(a)
	if ab != ba {
		t.Error("Add is order dependent")
	}
	if ab.Count != 2 {
		t.Errorf("count = %d, want 2", ab.Count)
	}
	if got := ab.Add(ce).Remove(a); got != (reconcile.Range{}).Add(b).Add(ce) {
		t.Error("Remove does not invert Add")
	}
	if got := ab.Merge(reconcile.Range{}.Add(ce)); got != ab.Add(ce) {
		t.Error("Merge differs from adding the elements")
	}
	if ab == (reconcile.Range{}).Add(a).Add(ce) {
		t.Error("ranges with different elements compare equal")
	}

	// Count wraps like Sum, so Remove from the empty range is undone by Add.
	if got := (reconcile.Range{}).Remove(a).Add(a); got != (reconcile.Range{}) {
		t.Errorf("Remove then Add on the empty range = %+v, want the empty range", got)
	}
}

// Order is bytewise on the address within a kind. These pairs differ from
// tuple order or from a locale collation, and SQL must agree with them.
func TestAddress_BytewiseOrder(t *testing.T) {
	addrs := []string{
		reconcile.EntityAddress("task", entity.Ref{ID: "X-1"}),
		reconcile.EntityAddress("task-list", entity.Ref{ID: "X-1"}),
		reconcile.EntityAddress("ticket", entity.Ref{ID: "X-1", Face: "draft"}),
		reconcile.EntityAddress("ticket", entity.Ref{ID: "X-10"}),
		reconcile.EntityAddress("ticket", entity.Ref{ID: "X-1"}),
		reconcile.EntityAddress("Ticket", entity.Ref{ID: "X-1"}),
	}
	slices.Sort(addrs)
	want := []string{
		"Ticket/X-1",
		"task-list/X-1",
		"task/X-1",
		"ticket/X-1",
		"ticket/X-10",
		"ticket/X-1@draft",
	}
	if !slices.Equal(addrs, want) {
		t.Errorf("order = %q, want %q", addrs, want)
	}
}
