---
id: RR-73CD
type: review-response
title: Reveal path comment claimed read-only but toV1 still emits _actions
finding: 'forWireHistoricalReveal → toV1 still calls computeActions against the live, unmarked ctx, so _actions is computed against a graph that (for a deleted entity) no longer describes it. The comment claimed the snapshot is read-only with no affordance maps. Not a data leak (booleans, reveal holder is an auditor, server re-authorizes writes), but the comment was inaccurate. The ordinary non-reveal path has the same toV1-_actions property.'
severity: minor
resolution: 'Corrected the forWireHistoricalReveal doc to state it carries the base _actions map from toV1 (advisory-only on a snapshot, a UI hint the server re-authorizes), and that the non-reveal history path shares that property. Left _actions emitting rather than restructuring the shared toV1 — proportionate to a minor cosmetic/accuracy finding.'
status: addressed
---
