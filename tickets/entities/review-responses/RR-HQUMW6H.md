---
id: RR-HQUMW6H
type: review-response
title: '[security] Rename away hides no new id'
finding: '[security] The lineage exemption also skipped the new id of a rename away from the asked id so the old id''s list could name a now-hidden entity.'
severity: significant
resolution: 'Only the earlier id of a rename into the asked id is exempt. Test: TestEntityChanges_RenameAwayChecksNewID.'
status: addressed
---

[security] The lineage exemption also skipped the new id of a rename away from
the asked id so the old id's list could name a now-hidden entity.
