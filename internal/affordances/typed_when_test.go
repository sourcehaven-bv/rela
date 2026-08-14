package affordances_test

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
)

// statusWritable reads the (sparse) Writable verdict for the `status`
// field. The maps are sparse: an absent key means the permissive default
// (writable); a present `false` means the `when:`-conditional grant
// evaluated false and DENIED the field. So writable = absent OR true.
func statusWritable(fv affordances.FieldVerdicts) bool {
	v, present := fv.Writable["status"]
	return !present || v
}

// TestResolver_IntegerWhen_NumericOrdering pins that an integer `when:`
// compares numerically after the Phase-2 adapter swap (integer -> IntType,
// bound via NewInt), NOT lexicographically. `priority > 9` must be true
// for 10 and 100 — the lexicographic "10" < "9" bug must not appear.
func TestResolver_IntegerWhen_NumericOrdering(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.priority > 9"
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := []struct {
		priority any
		writable bool
	}{
		{10, true},   // 10 > 9 numerically
		{100, true},  // 100 > 9 (lexicographic "100" < "9" would be wrong)
		{9, false},   // not strictly greater
		{8, false},   // 8 > 9 false
		{"10", true}, // string-stored int still coerces (RR-IRV2WJ)
		{int64(50), true},
	}
	for _, tc := range cases {
		fv := r.FieldVerdicts(ctxAs("alice"), ticket("T", map[string]any{"priority": tc.priority}))
		if got := statusWritable(fv); got != tc.writable {
			t.Errorf("priority=%v: writable=%v, want %v", tc.priority, got, tc.writable)
		}
	}
}

// TestResolver_DateWhen_TimeAndString is the RR-WHMVLW gate: a date
// `when:` must evaluate correctly whether the stored value is a time.Time
// (YAML auto-decodes an unquoted date scalar) or a string (quoted). If the
// binder missed the time.Time case it would bind Nil and silently deny.
func TestResolver_DateWhen_TimeAndString(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.due < '2026-06-01'"
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	jan15, _ := time.Parse("2006-01-02", "2026-01-15")
	aug01, _ := time.Parse("2006-01-02", "2026-08-01")

	cases := []struct {
		name     string
		due      any
		writable bool
	}{
		{"time.Time before (unquoted YAML shape)", jan15, true},
		{"time.Time after", aug01, false},
		{"string before (quoted YAML shape)", "2026-01-15", true},
		{"string after", "2026-08-01", false},
		{"missing date denies (fail-soft, not error)", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			props := map[string]any{}
			if tc.due != nil {
				props["due"] = tc.due
			}
			fv := r.FieldVerdicts(ctxAs("alice"), ticket("T", props))
			if got := statusWritable(fv); got != tc.writable {
				t.Errorf("due=%v: writable=%v, want %v", tc.due, got, tc.writable)
			}
		})
	}
}

// TestResolver_DateWhen_SemanticChange is the RR-782ULH gate. Phase 2
// changes date from StringType (lexicographic string compare of the raw
// stored value) to DateType (instant-granular time.Time ordering). This
// is an INTENTIONAL, security-relevant change on the ACL boundary. The
// observable difference for the affordance path is on values that
// previously string-compared but now must parse as a date:
//
//   - An UNPARSEABLE value (e.g. non-zero-padded "2026-1-5") used to be a
//     plain string compare; now it binds Nil and the grant fails closed
//     (denies). This is the desired direction on an auth boundary — an
//     ill-formed date should not silently grant via lexicographic luck.
//   - A DATETIME value with a time component now compares instant-granular:
//     "2026-06-01T08:00:00Z" is AFTER the bare date '2026-06-01' (midnight),
//     so `entity.due < '2026-06-01'` is false — the time-of-day is honored,
//     not truncated to a string prefix.
func TestResolver_DateWhen_SemanticChange(t *testing.T) {
	t.Parallel()
	p := policyFromYAML(t, `
roles:
  triager:
    fields:
      ticket:
        - field: status
          when: "entity.due < '2026-06-01'"
assignments:
  alice: triager
`)
	r, err := affordances.New(testMeta(t), newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := []struct {
		name     string
		due      any
		writable bool
	}{
		// Unparseable date -> Nil -> fail-closed deny (the intended shift).
		{"unparseable non-padded date denies", "2026-1-5", false},
		// Datetime instant-granular: 08:00 on Jun 1 is AFTER midnight Jun 1.
		{"datetime time-of-day honored (after)", "2026-06-01T08:00:00Z", false},
		// A datetime clearly before the bound stays writable.
		{"datetime before", "2026-05-31T08:00:00Z", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fv := r.FieldVerdicts(ctxAs("alice"), ticket("T", map[string]any{"due": tc.due}))
			if got := statusWritable(fv); got != tc.writable {
				t.Errorf("due=%q: writable=%v, want %v", tc.due, got, tc.writable)
			}
		})
	}
}
