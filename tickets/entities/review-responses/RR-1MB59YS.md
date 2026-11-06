---
id: RR-1MB59YS
type: review-response
title: Write rule outcome discloses visible:-hidden values
finding: Whether a write is refused depends on values the writer may not read, giving an oracle (security review).
severity: significant
resolution: appbuild warns at boot for each enforced rule that reads a property a role can hide (WriteRuleVisibilityConflicts, same pattern as query scopes). Documented in the metamodel guide.
status: addressed
---
