---
id: RR-S8LX1S
type: review-response
title: 'Dialog values rendered as [object Object] and other polish'
finding: 'clearConfirmMessage used String(value), so an array showed as a comma-run and an object as [object Object] - on the one screen where the user weighs what they are about to lose. Separately, applyHidePolicy returned early when autosave was not yet wired, silently skipping a clear the user had already approved. The fireDue doc comment also overclaimed atomicity: an unrelated field edited while a dialog is open still saves on its own.'
severity: minor
resolution: 'Values now render legibly (arrays joined, objects as JSON). The early return releases the retained copy so an approved clear cannot silently un-happen. The atomicity comment now states its real scope: it makes an approved DECISION atomic, it does not freeze the form.'
status: addressed
---
