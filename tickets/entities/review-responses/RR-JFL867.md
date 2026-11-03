---
id: RR-JFL867
type: review-response
title: Decorations rebuilt on every transaction
finding: The decorations prop walks the document on every transaction, including selection changes.
severity: nit
resolution: Decorations now live in plugin state and are rebuilt only when the document, the busy marks or the label change.
status: addressed
---
