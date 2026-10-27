---
id: RR-1FHO2M
type: review-response
title: Field-level denial of a handoff/claim set does not refuse the post; docs claim it does
finding: 'The output pre-check (internal/realtime/outputs.go:318 -> internal/appbuild/realtime_outputs.go:212-226) asks only AuthorizeUpdate on the row. A poster who has update on the anchor but whose fields: grant excludes the role property passes the pre-check. The move is stored and the set then fails with field_denied. In the guide example, a member with row update but no write on owner can post claim in handed-over. The session moves to contained with owner unchanged (the handoff target who never accepted), and this silences the not-accepted 4h escalation. GUIDE-realtime.md:702-703 (and docs/realtime.md) states "A poster without it is refused with session_output_denied before anything is stored", which is false for field-level grants. AC20''s clerk row (internal/appbuild/realtime_handoff_test.go:31-35, 54-56) uses a role with no update at all, so the field-level case is untested. Fix: make the pre-check ask field write for each property a set names (for example AuthorizeUpdate on the row with the set''s property names applied, or acl field-write per name), so the refusal happens before the post. Add an AC20 row with update: [datalek] plus a fields: block that excludes owner, asserting 403 session_output_denied and an unchanged LastSeq. If the pre-check stays row-only, correct the guide and the plan AC instead.'
severity: significant
resolution: 'The pre-post check now also runs the field gate on each set: output (literal value or nil for a template); a member with row update but no write on the role property gets 403 session_output_denied and nothing is stored; AC20 test row added'
status: addressed
---
