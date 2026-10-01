package affordances

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staticGrantsCallers are the only non-test files allowed to call
// WithStaticGrants. The flag widens every grant the resolver decides, so a
// new caller must be an offline analysis with no request behind it, added
// here with a reason.
var staticGrantsCallers = map[string]string{
	"internal/affordances/static.go":     "the definition",
	"internal/cli/classification_acl.go": "rela acl audit: worst-case views, no request",
}

func TestWithStaticGrants_CallersAllowlisted(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"internal", "cmd"} {
		walkErr := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if _, ok := staticGrantsCallers[rel]; ok {
				return nil
			}
			data, err := os.ReadFile(path) // #nosec G304 -- walking the module's own source tree
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "WithStaticGrants(") {
				t.Errorf("%s calls affordances.WithStaticGrants; only offline analysis may (see staticGrantsCallers)", rel)
			}
			return nil
		})
		if walkErr != nil {
			t.Fatal(walkErr)
		}
	}
}
