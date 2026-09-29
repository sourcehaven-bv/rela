---
id: RR-J30VJF
type: review-response
title: Edit selector depends on the keyboard-hint layout
finding: /^\s*Edit\b/ relies on the kbd hint following the label.
severity: nit
reason: The Edit link has no stable class or data attribute and adding one is a product change, which this ticket excludes. The selector is scoped to .desktop-actions and confined to FacesPage.
status: wont-fix
---

/^\s*Edit\b/ relies on the kbd hint following the label.
