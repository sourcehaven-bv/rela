---
id: RR-JE2G14
type: review-response
title: 'Mandatory permission: on elevated docs is necessary but not sufficient — an empty/unknown permission and the ReadOnlyACL arm both need pinning'
finding: |-
    The ticket's rule ('permission: required when elevated, config error at load') is right but underspecified in three ways that decide whether it actually fails closed.

    (1) NO-POLICY MODE. gateDocumentPermission returns true when docCfg.Permission == '', and readGateFromContext hands back nopReadGate under NopACL and under --read-only, whose HoldsPermission returns true. So on a project with NO acl.yaml, an elevated document with a permission: set is served to everyone anyway — the permission names a capability nothing can withhold. Requiring the field at load does not help here: the field is present, the gate just cannot deny. TKT-TXDK8U's commit message calls out exactly this trap (RR-CWWJGW shape: 'a predicate written against the gate alone fails open'). The ticket must say what an elevated document does when no policy is configured. Refusing to serve it — or refusing to load the config — is defensible; silently serving company-wide data is not.

    (2) UNKNOWN PERMISSION STRING. Nothing appears to validate that a document's permission: names a permission any role actually grants. `permission: repot:sales` (typo) is a permission no principal holds, which fails closed — fine. But the inverse, a permission granted to more roles than the operator believes, fails open silently. Validation should at minimum check the string against acl.yaml's known permissions when a policy is configured, as the nav permission: work presumably faced too.

    (3) --read-only. Commands are denied outright under --read-only; nav entries are deliberately SHOWN. Which applies to an elevated document? It is a read operation (so --read-only should permit it) that carries a write-shaped capability (so the command precedent says deny). The ticket must choose explicitly rather than inherit whichever arm the implementation happens to hit.
resolution: |
  Resolved by PLAN-1DETM0.
  
  (1) FAIL-OPEN GATE. Confirmed exactly as the finding describes: nopReadGate
  .HoldsPermission returns true (readgate.go:135) and readGateFromContext hands
  back nopReadGate under BOTH NopACL and ReadOnlyACL, so the existing
  gateDocumentPermission -- written against the read gate -- fails open. Harmless
  for a non-elevated document (content is ACL-bounded anyway); the whole boundary
  for an elevated one.
  
  Fix: a new authorizeElevatedDocument, a CLOSED SWITCH on the ACL implementation
  mirroring authorizeCommand (commands.go:84-119), whose godoc documents this exact
  trap as live bug RR-CWWJGW. Arms: nil -> deny; NopACL -> deny (see below);
  ReadOnlyACL value AND pointer -> deny; *Declarative -> nil-check then Permission
  non-empty AND held; default -> deny. Both value and pointer forms matched
  explicitly, because matching only the value form was the one-'&' bypass.
  
  (2) NopACL. Deliberate DIVERGENCE from authorizeCommand, whose NopACL arm grants
  to preserve pre-ACL behavior. An elevated document has no pre-ACL behavior to
  preserve -- the feature is new -- so granting merely creates a configuration in
  which the only boundary is inert. Decision: refuse at CONFIG LOAD ('elevated
  documents require a configured acl.yaml') rather than 403 at request time; an
  invalid configuration should not start.
  
  (3) --read-only. Denied, via the ReadOnlyACL arm.
  
  (4) Unknown permission string: validation should check it against acl.yaml's
  known permissions when a policy is configured. Carried into implementation.
  
  Pinned by AC7 (table-driven over every ACL impl incl. face forms) and AC8
  (NopACL refusal).
severity: significant
status: addressed
---
