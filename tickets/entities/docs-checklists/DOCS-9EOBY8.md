---
id: DOCS-9EOBY8
type: docs-checklist
title: 'Documentation: _redacted wire signal (BUG-MLT9DE / DEC-T0XIWQ)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new wire field — `v1.Entity.Redacted`
(`internal/apiwire/v1/responses.go`) explains what it is, *why* it exists
(absence is ambiguous; a write surface must tell redacted from unset), the
names-not-values disclosure boundary, and the closed-world present/absent semantics.
- [x] Godoc on `redactedPropertyNames` — documents the empty-not-nil contract
and why it takes resolved verdicts rather than re-resolving.
- [x] TSDoc on `isPropertyRedacted` — states plainly that inferring redaction
from absence is the bug, and why `undefined` answers `false`.
- [x] Inline comment on `affordanceVisible` explaining the edit/create split and
why create mode's signal is sound where edit mode's inference was not.
- [x] Inline comment on the `isEdit` early return in `handleSubmit` recording
that it is load-bearing for the redaction/data-loss argument, with an explicit
instruction for anyone adding a bulk edit submit later.

## Project Documentation

- [x] `docs/data-entry/api-reference.md` — the hidden-fields section documented
the unsound inference **as the contract** ("a field declared in data-entry.yaml
is rendered only if it appears in `_fields` OR `properties`"). Rewritten:
describes `_redacted`, states "never infer redaction from absence" in bold,
documents the closed-world semantics and the names-vs-values disclosure
boundary, and adds `_redacted` to the wire shape example.
- [x] ~~Metamodel / ACL guide updates~~ (N/A: no policy-authoring surface
changed — `visible:` semantics are unchanged, only how they are reported.)
- [x] ~~CLAUDE.md updates~~ (N/A: no new architectural rule; the existing
field-level ACL rule already states that `visible:` hides values only and makes
no claim to conceal which properties exist. This change makes the wire
consistent with that rule rather than amending it.)

## Decision Record

- [x] DEC-T0XIWQ records the decision, the rejected alternative (client-only
fix) and why, the security analysis, and the implementation-time correction
about the write path.

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: additive wire field; no user-facing
migration step. The user-visible effect is that a previously-invisible field now
appears, which needs no instruction.)
