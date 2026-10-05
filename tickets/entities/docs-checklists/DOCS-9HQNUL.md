---
id: DOCS-9HQNUL
type: docs-checklist
title: 'Docs: Schema property labels render on generic entity details'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] ~~Exported functions/types have godoc~~ (N/A: no exported function or type was introduced; `PropertyDef` received an optional schema field, consistent with its existing documented fields)
- [x] ~~Non-obvious decisions explained in comments~~ (N/A: label serialization and display fallback are direct and follow existing schema patterns)
- [x] ~~Package docs updated if package purpose changed~~ (N/A: package purpose unchanged)

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new project convention)
- [x] docs/metamodel.md updated for changed behaviour
- [x] ~~Architecture docs updated~~ (N/A: no package boundary, dependency, or wiring change)

Updated the canonical metamodel guide at
`docs-project/entities/guides/GUIDE-metamodel.md` with the optional property
`label` option, then regenerated `docs/metamodel.md` using `just docs`.

## External Documentation

- [x] ~~README updated~~ (N/A: schema option is documented in the metamodel guide)
- [x] ~~CLI reference updated~~ (N/A: no CLI change)
- [x] ~~API docs updated~~ (N/A: the `_schema` response reflects the schema label field; its generated response shape is not separately documented)
