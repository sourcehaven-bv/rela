---
id: RR-HMU95
type: review-response
title: config.validateName duplicates the validation rule set; will drift
finding: internal/config/config.go:85-112 has a copy of the validateKey rules, and its comment now references a state.validateKey symbol that no longer exists. Future rule changes must be remembered in two places.
severity: minor
reason: Fold into TKT-K3YYE (arch lint + shared validation) rather than bloating this PR. That ticket is already scoped to audit all path-handling callers; refactoring config.validateName to delegate to storage.RootedFS.resolve (or a shared helper) fits there. For now, config.validateName continues to work; the comment drift is cosmetic.
status: deferred
---
