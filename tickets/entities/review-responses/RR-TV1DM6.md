---
id: RR-TV1DM6
type: review-response
title: 'PR-A minors: idTaken doc, emitFamilyDeleted param, per-state event types, watcher naming, from_pointer parse, defaultTailKey'
finding: 'Six code-quality findings: (8) idTaken''s except widened from exact-key to case-folded-bare-id without the doc saying so; (9) emitFamilyDeleted took a redundant id param; (10) rename/delete events used family[0]''s type though load tolerance permits a mistyped state; (11) entityIDFromPath returned a state key under a name that says id, and the watcher parsed the stem twice per event; (12) readRelationFile parsed from_pointer frontmatter that loadRelationMeta immediately overwrote — a second source of truth entities deliberately avoid; (13) six raw from--type--to concatenations at the default-tail-only Get/Update/Delete sites relied on the reader knowing the Step-1 addressing decision.'
severity: minor
resolution: 'All addressed: idTaken docs state the family-wide case-folded semantics (both backends); emitFamilyDeleted drops the id param and uses per-state meta.ID/Type; rename events use per-state types; entityStemFromPath rename + entityIdentityFromPath now returns (id, pointer, type, ok) parsing once (encrypted-shell path also stamps the pointer and stats the right file); the from_pointer frontmatter parse is removed with a comment marking the key write-only human documentation (filename/index sole authority, mirroring entities); defaultTailKey helper added in both backends with the Step-1 rationale.'
status: addressed
---
