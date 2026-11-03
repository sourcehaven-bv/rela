package archguard

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

// mentionIndexOwners may name store.MentionIndex: the store packages define
// and implement it, and visibility is its one gated reader.
var mentionIndexOwners = []string{"internal/store/", "internal/visibility/"}

// mentionIndexUses returns every reference to the raw mention index: the
// MentionIndex type or a MentionSources call.
func mentionIndexUses(fset *token.FileSet, file *ast.File) []token.Position {
	var found []token.Position
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if name := sel.Sel.Name; name == "MentionIndex" || name == "MentionSources" {
				found = append(found, fset.Position(sel.Sel.Pos()))
			}
		}
		return true
	})
	return found
}

// TestMentionIndexOnlyThroughVisibility pins that backlinks are read through
// visibility.Resolver.MentionedBy (TKT-Q6AKUN). The raw index lists every
// mentioning row whatever the reader may see, so a surface calling it
// directly would disclose hidden entities.
func TestMentionIndexOnlyThroughVisibility(t *testing.T) {
	for path, uses := range scanTree(t, repoRoot, scannedRoots, mentionIndexUses) {
		owned := false
		for _, prefix := range mentionIndexOwners {
			owned = owned || strings.HasPrefix(path, prefix)
		}
		if !owned {
			t.Errorf("%s: %d use(s) of the raw mention index, first at %s; "+
				"read backlinks through visibility.Resolver.MentionedBy", path, len(uses), uses[0])
		}
	}
}
