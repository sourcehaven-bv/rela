---
id: RR-JILCGF8
type: review-response
title: 'Design: unrepresentable read query and local-role shapes'
finding: A ReadQueryResult with no branch set must be an error; local-role predicates must pass the elementnaive shape check.
severity: minor
resolution: 'A result with no branch is an error. The real-policy tests use a role_relations role with a visible: grant.'
status: addressed
---
