package pgstore

import "testing"

func TestTrigramIndexable(t *testing.T) {
	for needle, want := range map[string]bool{
		"":         false,
		"ab":       false,
		"a b":      false,
		"x-y":      false,
		"1.2":      false,
		"c++":      false,
		"ééé":      false, // non-ASCII: pg_trgm's word characters depend on the locale
		"abc":      true,
		"gateway":  true,
		"tsk-0367": true,
		"a 123":    true,
		"ab-cd-ef": false,
	} {
		if got := trigramIndexable(needle); got != want {
			t.Errorf("trigramIndexable(%q) = %v, want %v", needle, got, want)
		}
	}
}
