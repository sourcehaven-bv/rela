---
id: RR-5B3SWG
type: review-response
title: Drop onto a collapsed column is documented but not pinned
finding: The docs say a card cannot be dropped on a collapsed column; nothing tests it.
severity: significant
resolution: Added e2e test 'a card dropped on a collapsed column stays where it was' (real pointer drag onto the rail). The existing RlBoard DragAndDrop play test already pins that the keyboard move skips the collapsed Postpone column.
status: addressed
---
