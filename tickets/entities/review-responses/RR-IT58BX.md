---
id: RR-IT58BX
type: review-response
title: E2E save assertion passes without the fix
finding: Saves are deltas, so the stored set after adding a link is the same with or without the fix. Only the tile assertion guards the bug.
severity: significant
resolution: The spec now asserts the edit URL keeps ?world=, and removes the draft-only link in the form and asserts it is gone. That removal is only possible when the form loaded the link.
status: addressed
---
