package piles

import "github.com/Sourcehaven-BV/rela/internal/entity"

// FirstUnique collapses duplicate refs, keeping the first, and keeps at most
// limit of them.
func FirstUnique(refs []entity.Ref, limit int) []entity.Ref {
	seen := make(map[entity.Ref]bool, len(refs))
	out := make([]entity.Ref, 0, min(len(refs), limit))
	for _, r := range refs {
		if len(out) == limit {
			break
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
