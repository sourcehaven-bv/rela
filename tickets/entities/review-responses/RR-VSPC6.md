---
id: RR-VSPC6
type: review-response
title: Add metamodel-vs-filesystem guard to prevent silent folder-name drift
finding: The original bug was silent because fsstore inferred type from folder name (guide/ -> guid) with no validation. A startup/lint check that every folder under entities/ must match a defined type's plural (and every defined type with instances must live under its plural) would have caught this on first rela refresh instead of waiting for a human to notice misclassified entities. The default convention (type + "s") from fsstore.go:241-248 is a deterministic source of truth to assert against. Fits naturally as a validator in internal/validation/ or a migration.Detect rule. Out of scope for this commit.
severity: nit
reason: Valid follow-up but explicitly out of scope for this xs chore ticket (ticket scope picked "just move the files", not guardrail). Should be filed as a separate ticket against FEAT-CO4YP / store-backends with its own design review.
status: deferred
---
