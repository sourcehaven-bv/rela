---
id: IMPL-73C6B2
type: implementation-checklist
title: 'Implementation: historical redaction fails closed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] `acl.PermHistoryReadRedacted = "history:read-redacted"` constant beside PermHistoryRead; free-form permission (no registry) — grantable via a role's `permissions:` list
- [x] `affordances.WithHistoricalSubject(ctx)` marker + `isHistoricalSubject`; `bindings.outgoingCounts` returns no edges under the marker so has_relation/count_relations fail closed
- [x] Role resolution under the marker is globals-only (resolveViaDeclarative skips ForEntity's live local/ancestor probes); FieldVerdicts applies a type-level closed-world (typesWithVisible) so a reduced role set fails closed, not all-visible (RR-73CA)
- [x] `serveHistoryVersion`: ordinary reader → historical marker + forWire (fail closed); `history:read-redacted` holder → `forWireHistoricalReveal` (skips strip, OVERRIDE reveal)
- [x] `forWireHistoricalReveal` serializer path (toV1 without stripHiddenProperties), gated by the handler on the reveal permission
- [x] Relation history handler: comment marking the seam so TKT-B1F5Q1 wires the same fail-closed rule when it adds relation `visible:`
- [x] Docs: `history:read-redacted` section in GUIDE-acl-security (regenerated to docs/acl-security.md)

## Quality

- [x] `just lint` clean; `just arch-lint` clean (no new package boundary)
- [x] `just coverage-check` PASS (76.8% total)
- [x] Default + `-tags postgres` builds green (bleve/pgx separation intact)
- [x] Full default suite green; touched packages + DB-gated pgstore suite green under postgres tag
