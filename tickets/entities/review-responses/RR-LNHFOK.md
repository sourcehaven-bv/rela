---
id: RR-LNHFOK
type: review-response
title: Only one ACL mode covered
finding: The five type switches PR 2 moves branch on the ACL implementation; a Declarative-only corpus cannot see them.
severity: critical
resolution: 'Plan: variants for Declarative, NopACL, ReadOnlyACL, nil, and reject principals with and without a JWT gate.'
status: addressed
---
