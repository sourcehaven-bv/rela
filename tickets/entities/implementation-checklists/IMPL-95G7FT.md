---
id: IMPL-95G7FT
type: implementation-checklist
title: 'Implementation: Sync 1/5: shared canonical entity/relation serializer + content hash'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written — cross-backend decode test (YAML vs JSONB paths) in roundtrip_test.go is the linchpin; real fsstore/pgstore round-trips asserted in those stores' own suites when the hash is wired in (sub-tickets 2 & 4)
- [x] Happy path implemented — `HashEntity`/`HashRelation` in internal/canonical
- [x] Edge cases from planning handled — int/int64/float type-invariance, []string vs []any, nil vs empty map, body reflow idempotency, unicode, delimiter-collision
- [x] Error handling in place — no error paths (pure function); unknown types get a deterministic fallback sigil rather than panic

## Test Quality

- [x] Using fixture builders or factories for test data — `mk()` closures, table-driven
- [x] No hardcoded values in assertions when object is in scope — hashes compared, never literal digests
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end — `go test -race ./internal/canonical/` green
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `go test -race ./internal/canonical/` → ok
- coverage: 97.6% of statements (well above default floor 50)
- `golangci-lint run ./internal/canonical/` → 0 issues
- `go build ./...` → OK (no module-wide breakage)
- Linchpin AC met: `TestHashEntity_CrossBackendDecode` proves the same logical
entity decoded via YAML (fsstore path) and via JSONB+UseNumber+normalize
(pgstore path) hashes identically, across scalars, whole/fractional numbers,
lists, nested maps, unicode, multiline body, empty props.
- `fsstore/echo.go:46 hashContent` left untouched (verified not modified).

## Quality

- [x] Code follows project patterns — package doc explains the why; matches `any` convention; named delimiter constants
- [x] Checked for DRY opportunities — integer-width cases folded into `reflectInt`/`reflectUint`; delimiters named once
- [x] No security issues introduced — pure function, sha256; delimiters chosen to prevent value-smuggling collisions
- [x] No silent failures — unknown-type fallback is deterministic and documented, not a swallow
- [x] No debug code left behind
