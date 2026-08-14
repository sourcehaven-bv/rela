---
id: RR-OFWA2Q
type: review-response
title: -B with an empty file silently became "no updates specified"; --clear-body conflict checked content not flags
finding: Two adjacent CLI warts. (1) getBodyContent TrimSpaces, so `-B empty.md` (or whitespace-only) yielded "", left patch.Content nil, and with no other flags produced 'no updates specified' despite the operator explicitly naming a file. Pre-existing behaviour (old code gated `changed` on bodyContent != ""), so not a regression — but adding --clear-body made it MORE confusing, since `-B empty.md` and `--clear-body` now differ for no reason a user can infer. (2) The mutual-exclusion guard tested the RESOLVED content (`c.ClearBody && bodyContent != ""`), so `--clear-body -B empty.md` slipped through silently instead of erroring.
severity: minor
resolution: '(1) An explicitly named body file is now honored even when empty or whitespace-only: naming a source is an explicit instruction, and silently degrading it to an error about supplying nothing is baffling. Clearing deliberately is what --clear-body is for. (2) The conflict check now tests the FLAGS (c.Body != "" || c.BodyFile != "") rather than resolved content, so the empty-file combination is caught. Both pinned: TestUpdateCmd_EmptyBodyFileIsHonored and a table-driven TestUpdateCmd_ClearBodyConflictsWithBody covering --body and --body-file. Documented in cli-reference (and its docs-project source).'
status: addressed
---
