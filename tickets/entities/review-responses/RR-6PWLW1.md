---
id: RR-6PWLW1
type: review-response
title: Restore got delete wording and self-loop denial
finding: The neutral reason said delete on a restore, and the soft-deleted entity's own faces were not readable through the store, so its own edges counted as hidden.
severity: minor
resolution: authorizeRestore seeds edgeVisibility with the marked rows; reason is now 'a relation blocks this operation'.
status: addressed
---
