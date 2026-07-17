---
id: RR-UECSPI
type: review-response
title: change fired on every blur, not only on actual edit
finding: The element dispatched 'change' on every CodeMirror blur, unlike a native <textarea> which fires change on blur only if the value changed since focus. Consumers wiring autosave/dirty-tracking to 'change' would get spurious saves on every click-away.
severity: significant
resolution: Added a focus snapshot (_valueAtFocus); blur dispatches 'change' only when editor.value() differs. Covered by 'does NOT dispatch change on blur when nothing was edited' + the positive edit test.
status: addressed
---
