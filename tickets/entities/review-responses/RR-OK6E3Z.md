---
id: RR-OK6E3Z
type: review-response
title: The innermost-vs-shallowest item rule was undocumented
finding: The item lookup returns the innermost enclosing list item, so a cursor in a nested child toggles the child rather than the outer list. Defensible and consistent with the active-state probe, which also stops at the first match walking up, but undocumented - the next reader would reasonably assume the outermost list is the target.
severity: minor
resolution: 'The doc comment now states both rules and why they point in opposite directions: a cursor resolves INNERMOST so the button acts on the item its pressed state describes, while a range resolves SHALLOWEST and does not descend, so selecting a parent and child changes one level rather than two. A test pins the cursor case (`- parent\n  - child` toggles the child).'
status: addressed
---
