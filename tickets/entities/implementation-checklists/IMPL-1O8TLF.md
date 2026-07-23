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

- **Stage 1 DONE** (commit 38b2b58): `internal/cmdexec` (extracted safe-exec core; attachment.CmdRunner wraps it, attachment suite green), `internal/transform` (Registry, Engine, Renderer, built-in EntityRenderer), metamodel `transforms:` map + `validateTransforms` (from/command/produces-MIME, RR-VYTL35), `rela render` CLI command. arch-lint + golangci-lint clean on all touched pkgs; full `internal/...` suite green. Covers AC1 (loader table test incl. CRLF), AC7 (missing-binary via Probe), and the entity-renderer half of AC3/AC8.
- **Stage 2 TODO**: data-entry `GET /api/transforms` + entity export endpoint (getVisible, 404 on deny) + download hardening (nosniff/sandbox CSP/no-store/sanitized filename, RR-C3M3BR); frontend "Export as ▾". Covers AC2, AC3, AC4.
- **Stage 3 TODO**: list export (dataentry list renderer, ACL neighbor gate, RR-T3PDHN) + cap/truncation (RR-6ZDPTQ). Covers AC5, AC6.
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
