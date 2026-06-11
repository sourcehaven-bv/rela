---
id: DOCS-Z3XV
type: docs-checklist
title: 'Documentation: predicate-backed affordances'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious — package doc on
`internal/affordances`; consumer-side `RelationLookup` interface documented at
the call site; coercion fail-soft choices documented on `coerceList`; the
nil-program "unconditional grant" sentinel and the `//nolint:nilnil` rationale
documented on `compile`.
- [x] Function/type docs if public API — `Resolver`, `New`,
`FieldVerdicts`/`RelationVerdicts`, the acl.yaml grant types
(`FieldGrant`/`OptionGrant`/`RelationGrant`), and `Policy.HasAffordanceGrants`
all carry doc comments.

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level surface change)
- [ ] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern;
the consumer-side-interface and capability-bundle patterns the code follows are
already documented there)
- [x] ~~Help text accurate~~ (N/A: no CLI command changes; the only
knob is the existing `RELA_AFFORDANCE_PROFILE` env var, documented in
security.md + api-reference.md)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: project has no CHANGELOG file)
- [x] API docs updated — `docs/data-entry/api-reference.md` verdict-
source table now documents the policy-backed source and links to the security
model; `docs/security.md` gained the full "Field- and relation-level
affordances" reference (acl.yaml schema, predicate language, semantics, profile
selection).
