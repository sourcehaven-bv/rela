---
id: DOCS-CSD6FDM
type: docs-checklist
title: 'Docs: TKT-DTUAG3U'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why decimal is not `StringShaped`, why a decimal is emitted as text by the number widget, why `unique` is refused, why the predicate engine has no decimal division
- [x] Function/type docs if public API: godoc on `PropertyTypeDecimal` (stored form, comparison), `ParseDecimal`, `DecimalText`, `CompareDecimals`, `IsNumericType`, `predicate.Decimal`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela schema` prints `Scale:`; the number-type suggestion names `decimal`

## User-facing Documentation

- [x] `GUIDE-metamodel.md` (type tables, `scale` option, new "Decimal numbers" section with stored form, comparison, expressions and the not-yet-supported list, range and currency notes, roll-up limits), `GUIDE-data-entry.md` (widget table, summaries, currency), `GUIDE-cli-reference.md` (numeric comparisons), `GUIDE-data-migration.md` (`convert` to decimal); regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL or input-handling change beyond value validation)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI maps decimal to `string` with format `decimal`; `/_schema` and MCP `schema` carry `scale`
