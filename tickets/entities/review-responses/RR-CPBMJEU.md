---
id: RR-CPBMJEU
type: review-response
title: "App Info is hidden, not protected"
finding: "The card is gated on can_configure, which needs the Configure space; the counts it shows come from the schema every user loads."
severity: significant
reason: "The card shows only type and relation counts that every user can see elsewhere, so hiding it is the intent. can_configure is the only signal the SPA has for an admin; a deployment without the Configure space has no admin view to show it in."
status: wont-fix
---
