---
id: RR-KTAK7N
type: review-response
title: Whole-valued floats diverge fs vs pg (f:2 vs i:2)
finding: 'REPRODUCED. A frontmatter value `ratio: 2.0` decodes via yaml.v3 to float64(2) (yaml does NOT fold .0 to int). The same logical value in Postgres: JSON-marshals float64(2) to `2`, and pgstore normalizeJSONNumbers (entity.go:538-547) folds it to int(2). So fs emits f:2, pg emits i:2 — different hashes for the same record. Any property typed with a trailing .0 (versions, scores like 5.0, ratios) breaks sync. canonical.go:224-230 writeFloat godoc actively claims the OPPOSITE (''both backends normalize whole numbers to int'') — the comment is wrong and must not lull a maintainer. FIX: a whole-valued float must emit i:N not f:N (symmetric fold to pg). Better: normalize value types at one boundary (see L2).'
severity: critical
resolution: 'Added single-boundary normalize(): a whole-valued, exactly-representable float folds to int64 (normalizeFloat), matching pgstore''s 2.0->2. Fractional floats stay float64. Wrong writeFloat doc comment deleted. Regression TestHashEntity_WholeFloatEqualsInt asserts 2.0==2 and 2.5!=2. NOTE: fuzzing then surfaced a deeper precision bug (large whole floats >2^53 lose precision converting to int64) — fixed by only folding whole floats to int64 below 2^53 (maxExactInt), folding larger values on BOTH sides to the same lossy float64 so they still agree. Recorded as a permanent fuzz corpus seed.'
status: addressed
---
