---
id: TKT-GM3IQ7
type: ticket
title: Fix dangling docs/security.md references reintroduced after the server-security consolidation
kind: docs
priority: low
effort: xs
status: done
---

TKT-Q0TCW1 renamed `docs/security.md` → generated `docs/server-security.md`. A
later PR (the `rela acl audit` linter work) was written against the
pre-consolidation tree and reintroduced three references to the now-deleted
path:

- `docs-project/entities/guides/GUIDE-acl-security.md:64` and `:479`
(regenerates into `docs/acl-security.md`)
- `internal/aclaudit/tier_a.go:60` — an operator-facing audit *finding
message* (the `Fix:` text) pointing at a nonexistent doc

All three repointed to `docs/server-security.md`; docs regenerated. Follows up
TKT-Q0TCW1.
