---
id: RR-N0WRN5
type: review-response
title: stopEvent swallowed every event on the checkbox, including navigation keys
finding: '`stopEvent` returned true for any event whose target was the checkbox, including `keydown`. A user who tabbed to the checkbox and pressed an arrow key got nothing: ProseMirror never saw the key, so moving the selection out of the checkbox was dead.'
severity: minor
resolution: Narrowed to the two events the view actually handles, `mousedown` and `click`, so navigation keys reach ProseMirror. The comment says why it is not simply every event on the element.
status: addressed
---
