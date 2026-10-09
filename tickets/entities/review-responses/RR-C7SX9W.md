---
id: RR-C7SX9W
type: review-response
title: 'Security: rename trigger evaluated on the renamer''s redacted read'
finding: A condition on a field hidden from the renamer evaluated against a redacted entity, so it could fire where a save would not.
severity: significant
resolution: The rename hook reads raw, as the save cascade does. TestBackgroundAction_RenameDecidesOnStoredValues (mutation-checked).
status: addressed
---
