---
id: IMPL-6TAJ1U
type: implementation-checklist
title: 'Implementation: rela trace/path output ignores display_property (tracer builds titles from literal title)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestWriteTrace_TitleResolver: resolver-set, nil-fallback, nested-child; TestWriteTraceJSON pins Properties absent)
- [x] ~~Integration tests~~ (N/A: exercised end-to-end manually via `rela trace`; unit tests cover the output boundary)
- [x] Happy path implemented (tracer carries raw Properties; output.traceTitle resolves via TitleResolver)
- [x] Edge cases handled (nil resolver → literal title; nested children recurse; JSON stays raw)
- [x] Error handling: n/a (pure formatting)

## Test Quality

- [x] Reused the fakeTitleResolver + new idTitleResolver (per-ID) rather than hardcoding
- [x] Assertions specify only what matters (title substrings, Properties absence)
- [x] No hardcoded magic

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- Built `bin/rela`, scratch project with `persoon` (`display_property: "{voornaam} {achternaam}"`) leading a titled `project`.
- `rela trace from PERS-JV` → `PERS-JV Jeroen Vloothuis` (template rendered; was blank/literal before). `rela trace to PROJ-1` → same resolution upstream. `PROJ-1 Rela Platform` (literal title) unchanged.
- `rela trace from PERS-JV -o json` → NO `Properties` field (raw JSON schema unchanged); confirmed after the `json:"-"` fix.
- `rela trace path` runs cleanly (path text is ID/Type only, no title — unaffected).

## Quality

- [x] Follows project patterns (output-boundary resolution mirrors TKT-VHSHOB TitleResolver; tracer stays a pure reader — arch-lint clean)
- [x] DRY: reused the existing TitleResolver; traceTitle is a thin sibling of entityTitle
- [x] No security issues (git-crypt locked-title concern is table/mention-side, unaffected here; trace shows what the reader can read)
- [x] No silent failures
- [x] No debug code left behind
