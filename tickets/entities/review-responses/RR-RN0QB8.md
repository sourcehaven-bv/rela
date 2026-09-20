---
id: RR-RN0QB8
type: review-response
title: 'Self-edge is destroyed outright by create-then-delete'
finding: 'For A--t-->A the reversed triple is byte-identical to the source (FormatStateRef with an empty face is the bare id, face.go:110; Relation.Key documents the equivalence at entity.go:302-308). CreateRelation returns ErrConflict against the very edge about to be deleted (fsstore/relation.go:145-148), the tolerance falls through, and DeleteRelation removes it. Verified empirically on a memstore: survivors = 0. The plan named this hazard in an Edge Cases bullet but specified no guard, gave it no acceptance criterion, and placed it in a different section from the algorithm it endangers - so an implementer following the Approach section verbatim writes the destructive version.'
severity: critical
resolution: 'Promoted from an Edge Cases bullet to guard 2 of the pre-flight pass in the Approach section, where the algorithm lives: self-edges are skipped outright since reversal is identity, and they do not count toward Affected. Given its own acceptance criterion asserting the edge survives with properties and content intact, rather than being left to an implementer to remember from a different section.'
status: addressed
---
