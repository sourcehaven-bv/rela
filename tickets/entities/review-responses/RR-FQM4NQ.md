---
id: RR-FQM4NQ
type: review-response
title: uint64 > MaxInt64 diverges in type and precision
finding: 'REPRODUCED. Frontmatter `u: 18446744073709551615` decodes via yaml.v3 to uint64 → canonical emits i:18446744073709551615 (exact). The pg path JSON-marshals it, reads json.Number, Int64() fails (out of range), falls to Float64() → float64(1.8446744073709552e19) → canonical emits f:1.8446744073709552e+19. Different sigil AND lost precision. Divergent hash plus silent numeric corruption. Less common than dates but a real unsigned-64 value with total divergence. Same root cause as C2/C3/C4: the two backends hand the hasher different Go types for the same logical value. FIX via the single-boundary normalization (L2) with an explicit uint64 policy.'
severity: critical
resolution: normalize()/normalizeUint folds uint64 (and int64) above the exact-integer range to the SAME lossy float64 pgstore is forced to read from JSONB, so both backends agree (lossily but identically). Below 2^53 they stay exact int64. The maxExactInt boundary (2^53) is the float64 exact-integer limit. Same mechanism also fixed the large-whole-float precision bug fuzzing found (see RR-KTAK7N).
status: addressed
---
