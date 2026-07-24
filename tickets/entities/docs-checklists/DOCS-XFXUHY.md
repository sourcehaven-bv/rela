---
id: DOCS-XFXUHY
type: docs-checklist
title: 'Docs: export field-level ACL redaction via visibility.Reader (TKT-L9Q669)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] exportHandler/visReader/redactor godoc explains the seam routing and why (DEC-ZBI39P, RR references)
- [x] visibility.Redact godoc: no-row-gate contract, when to use vs Get
- [x] ExecuteDocument godoc: ctx/principal threading rationale
- [x] Singleflight key comment: why the principal participates (RR-2QSGLU)

## Project Documentation

- [x] docs/transforms.md — new §Access control: entity-level gate, field-level redaction (incl. title/neighbor fallbacks), export_render principal threading + the documented PR-3 residual (in-script reads operator-trusted until TKT-ZF2DTV)

## External / User-Facing Documentation

- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~README.md~~ (N/A)
