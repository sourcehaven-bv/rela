---
id: RR-ZXGPCU
type: review-response
title: 'documents fail open on permission: while commands fail closed — same config key, opposite defaults'
finding: 'Under a configured acl.yaml, a `commands:` entry without `permission:` is DENIED (commands.go:102-115, explicit ''fail closed: a policy is configured, this command is ungoverned''), while a `documents:` entry without `permission:` is ALLOWED. The two entries look identical in YAML and both ''run a script'', but behave oppositely. That asymmetry is invisible to an operator.'
severity: significant
resolution: 'Kept the asymmetry — it is justified, not accidental: a document''s content is bounded by the ACL-gated VisibleReader, so an ungated document leaks nothing, whereas a command shells out and its side effects are not ACL-bound. Flipping documents to fail-closed would make `permission:` mandatory ceremony on every standalone document. Documented the divergence explicitly in docs/data-entry.md under ''Gating a document'', side by side with commands and with the reason, so an operator writing both keys sees it stated rather than discovering it.'
status: addressed
---
