---
id: RR-RACANY
type: review-response
title: 'PR-C minors: CLI line dedup+ellipsis, per-family aggregation, IsIdentity accessor, content-word collision, per-store warning id, probe debug'
finding: '(5) the analyze-states line printed the count twice (Detail embedded it) and example lists shorter than the count had no ellipsis; (6) state-type-mismatch emitted one Count:1 finding per row, surprising Subject-grouping consumers; (7) ScopeIdentity''s dual spelling (""/"identity") made `scope == ScopeIdentity` an attractive wrong comparison with no safe identity-side accessor; (8) RelationDef.Scope''s "content" value and the sibling Content bool (markdown bodies) share a word with no cross-reference; (9) warnUndeclaredFaces fired per-Assemble with no store identifier (multi-tenant hosts get N indistinguishable lines); (10) probe errors were swallowed with zero trace. Leverage: (11) collectStateFamilies retained full EntityHeaders (live Properties maps, fs-fallback bodies) when only (face, type) is needed; (12) the mismatch check could collapse into the post-collection loop.'
severity: minor
resolution: 'All addressed: Detail no longer embeds the count and the CLI line prints `[code] subject: N row(s) — detail (e.g. …)` with an ellipsis when capped; mismatches aggregate per family; RelationScope.IsIdentity added with a godoc warning against constant comparison; cross-reference comments on both Scope and Content fields; the warning carries the project root; probe errors log at Debug; headers project to a two-field stateRow at collection; the mismatch check lives in the post-collection loop (which is also what fixed significant 3).'
status: addressed
---
