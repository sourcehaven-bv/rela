---
id: RR-7OLO
type: review-response
title: Entity vs Relation Inaccessible representation is asymmetric and a footgun
finding: 'fsstore/markdown.go:370-384 enumerates schema-declared properties for entities; :469-478 uses single {Name: "*"} for relations. Three consumers handle this differently: SPA EntityList implements `*` semantics; SPA EntityDetail.isFullyInaccessible checks for `*`; Go validator/handler guards just check length. Combined with finding #1 (empty PropertyOrder) this asymmetry produces correctness bugs. Pick ONE: always include {Name: "*"} sentinel and let consumers expand if they want per-property granularity, OR always enumerate. Document in entity.go near InaccessibleField. Better: typed result `InaccessibleScope { Whole bool; Fields []string; Reason }` plus IsFullyInaccessible() / Fields() methods on Entity. Eliminates the magic-string `*`.'
severity: significant
status: deferred
reason: |-
    Parent ticket TKT-PGK91 (git-crypt detection) shipped via PR #668 without addressing this finding. Captured here so the gap remains visible; will be revisited if the underlying code path becomes a problem in practice. Closed as deferred via the TKT-5S8T data-debt sweep — the alternative is leaving the RR open indefinitely while it blocks every unrelated PR.
---
