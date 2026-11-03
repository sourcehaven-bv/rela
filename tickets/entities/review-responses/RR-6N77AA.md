---
id: RR-6N77AA
type: review-response
title: invalid_message fallback ignores an open edit
finding: Approach 9 step 6 handles a place:'form' refusal of a one-click post by `composer.select(type)` plus formError. A one-click post is allowed during an edit ('a one-click post leaves it'), and select() returns false during an edit, so formError then shows inside the edit form of a different message. Handle this the same way as step 2 (ask to discard the edit, or toast the message when select returns false).
severity: minor
resolution: A form-type refusal of a one-click post during an edit is shown as a toast instead of inside the edit form.
status: addressed
---
