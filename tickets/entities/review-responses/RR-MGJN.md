---
id: RR-MGJN
type: review-response
title: 'Entity title rendering: list view shows ID as title, no lock; detail view inconsistent'
finding: 'DisplayTitle falls back to e.ID when title is empty — so V1Entity._title for an encrypted entity is the entity ID, indistinguishable from a deliberately ID-titled entity. EntityList.vue:653 / mobile-card-title at :645 renders getFormattedCellValue for the FIRST column WITHOUT consulting isCellInaccessible, so the title column displays the ID in plain text while subsequent columns show locks. Inconsistent rendering. Fix: branch the first column on isCellInaccessible too (mobile + desktop), OR have entityToV1 emit empty title for fully-inaccessible entities and let the SPA render a placeholder.'
severity: significant
status: deferred
reason: |-
    Parent ticket TKT-PGK91 (git-crypt detection) shipped via PR #668 without addressing this finding. Captured here so the gap remains visible; will be revisited if the underlying code path becomes a problem in practice. Closed as deferred via the TKT-5S8T data-debt sweep — the alternative is leaving the RR open indefinitely while it blocks every unrelated PR.
---
