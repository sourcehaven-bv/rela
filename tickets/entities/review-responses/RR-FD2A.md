---
id: RR-FD2A
type: review-response
title: 'Round 2: third converter site for GroupData.Entities + stale Technical Approach'
finding: |
  Two cleanups before implementation:
  - Significant: api_v1.go:3066-3077 has a third V1ViewEntity construction for group-card display (`grp.Entities`). buildSections never populates GroupData.Entities today (only .Rows for table display), so it's dormant — but a future producer would silently drop _props/_fields. Either delete the dead loop or apply the dumb-copy snippet there too.
  - Minor: PLAN's "Technical Approach" section (L125-150 of the round-1 draft) still describes the entity back-reference + IsHiddenForType helper — both explicitly reversed by RR-FD1A/RR-FD1E. AC section is authoritative and correct; the Technical Approach contradicts it. Next reader will get whiplash.
severity: minor
status: addressed
resolution: |
  - Group-entity loop: PLAN AC 4 amended to call out ALL THREE converter sites — `api_v1.go:3010` (top-level entities), `api_v1.go:3066-3077` (GroupData.Entities, currently dormant), plus any future loop must follow the same dumb-copy pattern. Implementation chooses delete-dead-code vs apply-snippet based on what's least surprising; recommend apply-snippet to keep parity if grouped cards ever ship.
  - Technical Approach: rewritten to match ACs — `buildSectionEntityData` helper, `copyVisibleProperties` filtered through `hiddenProperties`, precomputed `Fields` on `SectionEntityData`, no entity back-reference.
---
