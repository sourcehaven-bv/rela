---
id: RR-OITUUS
type: review-response
title: WithStaticGrants widens access with no mechanical guard
finding: '[security] affordances.WithStaticGrants is an ambient ctx flag that makes every when: pass in PolicyResolver.passes; only a comment kept it off request paths.'
severity: minor
resolution: 'Added internal/affordances/staticguard_test.go: scans internal/ and cmd/ non-test files and fails on any WithStaticGrants caller outside an allowlist (static.go and internal/cli/classification_acl.go), each with a reason.'
status: addressed
---
