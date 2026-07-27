---
id: DOCS-73C6B2
type: docs-checklist
title: 'Docs: historical redaction fails closed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on `acl.PermHistoryReadRedacted`, `affordances.WithHistoricalSubject` (describes the complete neutering: outgoingCounts, globals-only role resolution, type-level closed-world), `PolicyResolver.typesWithVisible`, `forWireHistoricalReveal`, and the `serveHistoryVersion` two-tier branch
- [x] ~~CLAUDE.md pattern update~~ (N/A: reuses existing ACL/serializer patterns; no new cross-cutting rule)

## Project Documentation

- [x] `docs/acl-security.md` (via docs-project GUIDE-acl-security source, regenerated): new "Historical field redaction fails closed (`history:read-redacted`)" subsection — the invariant, the deny-by-default rule, globals-only + type-level closed-world, and OVERRIDE reveal semantics
- [x] Relation-history seam documented in `relation_history_handler.go` for TKT-B1F5Q1 to inherit the rule

## External Documentation

- [x] ~~README / external docs~~ (N/A: security-hardening guide is the operator-facing surface, updated above)

**Docs verified:** acl-security.md regenerated with no unintended drift; the
documented invariant matches the implemented fail-closed behavior including the
role-resolution path (RR-73CA).
