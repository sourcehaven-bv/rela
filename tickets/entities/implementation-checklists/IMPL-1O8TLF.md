---
id: IMPL-1O8TLF
type: implementation-checklist
title: 'Implementation: Transform registry + view export (pdf/docx via external tools)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Development

- [ ] Unit tests written for new code
- [ ] Integration tests written (test full flow, not just units)
- [ ] Happy path implemented
- [ ] Edge cases from planning handled
- [ ] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [ ] Using fixture builders or factories for test data
- [ ] No hardcoded values in assertions when object is in scope
- [ ] Only specifying values that matter for the test
- [ ] Interpolated values constructed from objects, not hardcoded
- [ ] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [ ] Feature manually tested end-to-end
- [ ] Each acceptance criterion verified with test scenario from planning
- [ ] Edge cases manually verified

**Verification Evidence:**

Staged implementation. Progress:

- **Stage 1 DONE** (commit 38b2b58): `internal/cmdexec` (extracted safe-exec core; attachment.CmdRunner wraps it, attachment suite green), `internal/transform` (Registry, Engine, Renderer, built-in EntityRenderer), metamodel `transforms:` map + `validateTransforms` (from/command/produces-MIME, RR-VYTL35), `rela render` CLI. Covers AC1 (loader table incl. CRLF), AC7 (missing-binary Probe), entity-renderer half of AC3.
- **Stage 2 DONE** (commit 2e7b6f1): `GET /api/v1/_transforms`; `GET /{plural}/{id}/_export?transform=` — ACL-gated via getVisible (404 on deny), hardened download (nosniff/sandbox-CSP/no-store/sanitized filename, RR-C3M3BR), visible neighbor titles via the relation visibility gate. Frontend transforms api-client + ExportMenu in entity detail. Go export_test.go covers AC2, AC3, AC4 (+ unknown-transform, missing-param) with a `cp {in} {out}` fake transform; frontend transforms.test.ts. golangci-lint + arch-lint + full dataentry suite + frontend typecheck/lint/build all green.
- **Stage 3 TODO**: list export (dataentry list renderer, ACL neighbor gate, RR-T3PDHN) + cap/truncation notice (RR-6ZDPTQ). Covers AC5, AC6.
- **Stage 4 TODO**: Lua render override via the gated `_documents` path (RR-8C23IL). Covers AC8 Lua-override half.
- Docs + CLAUDE.md notes: pending final stage.

## Quality

- [ ] Code follows project patterns (check similar code)
- [ ] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [ ] No security issues introduced
- [ ] No silent failures (errors logged AND returned)
- [ ] No debug code left behind
