---
id: RR-FC1E
type: review-response
title: 'Minor concerns C3, N1-N5, L1-L3'
finding: |
  - C3: `handleRowPropertyApplied` walking sections is O(sections × rows). Build an index map.
  - N1: `:key` semantics defensive — document why.
  - N2: `_props` fallback (AC 4) is dead today on a current server — document.
  - N3: Memo cache invalidation on row spread-clone — spec the clone shape.
  - N4: Inaccessible-field interplay missing from AC list.
  - N5: Multi-instance global "saving" indicator — confirm intentional.
  - L1: Covered by RR-FC1A.
  - L2: Nominal OwnerRef type — defer.
  - L3: Single section-level orchestrator — defer.
severity: minor
status: addressed
resolution: |
  - C3: PLAN adds a memoized `Map<\`${type}/${id}\`, { sectionIdx, rowIdx }>` rebuilt per viewData change. handleRowPropertyApplied does O(1) lookup. AC 7 stress test asserts correctness under sort/group reorder.
  - N1: PLAN's :key code comment documents: "Defensive — guards against pathological cases where the same array slot gets reused for a different entity. In practice the row's id is stable across loadView."
  - N2: PLAN AC 4 + code comment: "Defensive — post-IHC7D server always sends `_props`. This branch handles legacy servers / cache states / future shape drift."
  - N3: PLAN AC 7 specs the clone shape: `{ ...section, entities: section.entities.map((e, i) => i === targetIdx ? nextEnt : e) }`. Cache invalidation tracked: cache is keyed on `ent` reference, the targetIdx entry's reference changes (correct), all other entries' references survive (correct).
  - N4: PLAN AC 1 amended to include "inaccessible fields disable inline-edit for that row" — same rule as the entry section.
  - N5: PLAN docs "by design — per-row indicators are the canonical signal; no global aggregation."
  - L2, L3: Deferred follow-up tickets if either becomes painful in practice.
---
