---
id: RR-VP45T87
type: review-response
title: Identity-source predicate duplicated
finding: The same predicate lived in main.go and configure.go.
severity: minor
resolution: Extracted hasIdentitySource, used by warnUnidentifiedACL and validateConfigEditing.
status: addressed
---
