---
id: RR-P6ZFSV
type: review-response
title: Merge domain undefined for keys the patch omits — automation-managed fields would conflict permanently
finding: 'Manager.UpdateEntity runs automations that mutate the entity in place before persisting (manager.go:590-593, PropertiesSet → e.SetString); the PATCH response and its fresh ETag reflect the post-automation state. This repo''s metamodel uses such automations heavily (status transitions, {{today}}). The plan says ''per property: theirs===base→ours; ours===base→theirs; all differ→conflict'' but NEVER defines the merge domain or what ''ours'' means for a key the patch omits — and autosave patches a SINGLE key ({properties: {[property]: value}}, useAutoSave.ts:313-315). If ''ours'' means the attempted PATCH body, then for an automation-managed field like updated_at: ours=undefined ≠ base, theirs=Bob''s new value → all three differ → spurious conflict on a field the user never touched, on every 412, for most entity types in this repo. FIX: specify explicitly that the merge domain is the union of keys in base and theirs, that ''ours'' for an omitted key means UNCHANGED (equal to base, NOT absent/deleted), and that an automation-managed field therefore cannot conflict. Absent this the implementation is a coin flip that produces permanent false conflicts.'
severity: significant
status: open
---
