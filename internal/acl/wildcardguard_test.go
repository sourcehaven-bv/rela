package acl

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// wildcardCompare matches a direct comparison against the "*" grant
// wildcard: `x == "*"` or `slices.Contains(list, "*")`.
var wildcardCompare = regexp.MustCompile(`== "\*"|, "\*"\)`)

// wildcardSites pins every direct "*" comparison outside wildcardCovers,
// per file, with the reason it may not use it. A grant site that matches a
// type against a role's list must use wildcardCovers or coversType, so
// "*" never covers a rela-managed type (TKT-BC5G8RP). A new comparison
// fails here until it is either routed through the helper or listed with
// a reason.
var wildcardSites = map[string]struct {
	count  int
	reason string
}{
	"managed.go":           {1, "wildcardCovers itself"},
	"ceiling.go":           {2, "load-time refusal of a wildcard in field and world ceilings"},
	"ceilingcompile.go":    {7, "client ceilings only narrow; a ceiling naming a managed type is refused at load"},
	"facedgrants.go":       {4, "load-time validation of faced write grants; grants nothing"},
	"policy.go":            {1, "builds a denial hint string"},
	"rolerelationreach.go": {2, "matches a schema relation's From list, not a grant"},
	"traversal.go":         {1, "a visible key \"*\" restricts fields; it grants nothing"},
	"visiblealias.go":      {1, "skips the wildcard key while checking names; grants nothing"},
	"worldgrant.go":        {1, "the branch then calls wildcardCovers"},
}

func TestWildcardSitesExcludeManagedTypes(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			code, _, _ := strings.Cut(line, "//")
			if wildcardCompare.MatchString(code) {
				got[f]++
			}
		}
	}
	for f, n := range got {
		want, ok := wildcardSites[f]
		if !ok || want.count != n {
			t.Errorf("%s: %d direct \"*\" comparisons, allowlist says %d; "+
				"match grants through wildcardCovers or coversType", f, n, want.count)
		}
	}
	for f, want := range wildcardSites {
		if got[f] == 0 && want.count > 0 {
			t.Errorf("%s: allowlisted but has no \"*\" comparison; shrink the list", f)
		}
	}
}
