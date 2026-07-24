---
id: RR-FJWIMT
type: review-response
title: run_as ships with no validation and no user-facing documentation
finding: 'The design is verified sound — readQuery returns DenyAll when no role grants read, so an unassigned run_as principal denies as intended and run_as cannot grant anything by itself. But it is a new user-facing YAML field with ZERO documentation: docs/scheduled-tasks.md documents name/script/every and is untouched; docs/acl-security.md and docs/lua-scripting.md are also untouched despite a semantic change to what every script can read. There is also no validation — a typo (system:diggest) silently produces a job that reads nothing, logging only a fail-closed warning: the same silent-empty failure mode as the entity_refs bug. A startup warning when run_as matches no acl.yaml assignment turns that into a diagnosable error.'
severity: significant
resolution: 'docs/scheduled-tasks.md gains an ''Identity and what a task can read (run_as)'' section: the identity-not-permission distinction, the paired acl.yaml example, the empty-reads-on-unassigned-identity failure mode called out as the thing to check when a task stops finding data, and the field-redaction limitation. docs/lua-scripting.md gains a ''What a script can read'' section with the per-path identity table and the behavioral consequences (absent-not-error, peer-gated relations, pruned traversals, unredacted update read). Startup validation of run_as against assignments is noted but not implemented — recorded as a follow-up rather than claimed.'
status: addressed
---
