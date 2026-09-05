package dataentry

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestChromePlaceholdersInSyncWithFrontend pins metamodel.ChromePlaceholders
// to the SPA's allowlist in frontend/src/utils/worldText.ts (TKT-5SZG2L,
// RR-VJYG4V). The Go side documents the names an operator may write; the TS
// side is what substitutes them. A name added to one and not the other
// renders literally on screen with no failure anywhere, so — like
// TestAppTokensCSSInSyncWithFrontend — both files are read off disk and
// compared to each other.
func TestChromePlaceholdersInSyncWithFrontend(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "utils", "worldText.ts"))
	if err != nil {
		t.Fatalf("read worldText.ts: %v", err)
	}
	m := regexp.MustCompile(`const KEYS[^=]*=\s*\[([^\]]*)\]`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("worldText.ts no longer declares `const KEYS = [...]`; update this test with it")
	}
	var ts []string
	for k := range strings.SplitSeq(string(m[1]), ",") {
		if k = strings.Trim(strings.TrimSpace(k), `'"`); k != "" {
			ts = append(ts, k)
		}
	}
	if got, want := strings.Join(ts, ","), strings.Join(metamodel.ChromePlaceholders, ","); got != want {
		t.Errorf("placeholder allowlists differ: worldText.ts KEYS = [%s], metamodel.ChromePlaceholders = [%s]",
			got, want)
	}
}
