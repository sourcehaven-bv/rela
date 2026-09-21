---
id: RR-83NRB1
type: review-response
title: 'Assorted minor findings: duplicate keys, redundant request, error-note attribution, per-row recomputation'
finding: 'Four smaller items from review: (1) `:key="item.id"` collides when /_search returns two faces of one id; (2) `clearType()` fired a redundant search when already unscoped, and `selectType` likewise when already on that scope; (3) the loading/error note rendered under the Types heading with no Entities heading, so a failed search read as the type picker breaking; (4) `isEmpty()` and `entityIndex()` were functions re-reading `typeItems.length` on every call, ~80 property reads per render at 20 rows, against the precomputation discipline frontend/CLAUDE.md mandates for dense surfaces.'
severity: minor
resolution: (1) key is now `entity:${id}:${idx}`, namespaced like the type rows already were, so a face duplicate cannot collide. (2) both mutators early-return on a no-op, pinned by 'does not fire a search for a no-op scope change'. (3) the loading/error branch now renders the Entities heading above the note when type rows are present, attributing the failure to the right section. (4) `isEmpty` and `typeCount` are computeds.
status: addressed
---
