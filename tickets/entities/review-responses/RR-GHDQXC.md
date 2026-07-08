---
id: RR-GHDQXC
type: review-response
title: Sibling loaders (dataentryconfig, acl) share the same whitelist-drift class
finding: internal/dataentryconfig/validate.go (validTopLevelKeys) and internal/acl/policy.go (knownPolicyKeys) use the same hand-maintained-whitelist-vs-struct pattern that caused BUG-5XIN07, with no reflection-parity test. Out of scope for this bug, but a follow-up should apply the same parity-test pattern there. Prevention material.
severity: minor
reason: Out of scope for this bug (reviewer explicitly flagged as follow-up material, not a defect in this change). Filed as TKT-ELX09J to apply the same reflection-parity test to internal/dataentryconfig/validate.go and internal/acl/policy.go.
status: deferred
---
