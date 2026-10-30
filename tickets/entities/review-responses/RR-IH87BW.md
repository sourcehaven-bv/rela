---
id: RR-IH87BW
type: review-response
title: The "roles-only entry" case does not test what the plan names
finding: 'internal/realtimecfg/default_in_test.go:143-147: the plan''s AC 2 asks for "an entry whose roles no caller holds". The case widens post to member and keys the entry on member, a role that callers hold. Derivation reads only raw on: keys, so the case covers nothing that "single entry" does not. The deviation is noted in the impl notes, but the name still claims the plan''s case. Fix: rename it to "entry limited to another role keys its state", or drop it and say in the notes that a role no caller holds cannot be written, because entry roles must be in post.'
severity: nit
resolution: Renamed the case to "entry limited to another role keys its state". A role no caller holds cannot be written, because entry roles must be in the type's post (noted in the impl notes).
status: addressed
---
