---
id: DOCS-C3OWWP
type: docs-checklist
title: 'Docs: Design tokens: spacing, radius, typography and elevation scales'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc/comments on new exported symbols
- [x] Non-obvious decisions explained with WHY, not WHAT

`frontend/src/styles/scales.css` carries a header explaining why it is a
*sibling* of `tokens.css` rather than an extension (tokens.css is copied
byte-identically into the Go binary and restricted to theme tokens), and why
`--font-size-dense` is named as a role rather than a `-md` ramp step.

`internal/dataentry/apps_test.go` documents why the contract test reads both
files off disk — an earlier version asserted Go against literals in the Go test
file, which only proved Go was self-consistent and let a CSS-side revalue pass
green.

## Project Documentation

- [x] `frontend/CLAUDE.md` updated
- [x] ~~CLAUDE.md (root)~~ (N/A: the convention is frontend-scoped)

Added "Design tokens: two files, different contracts" — a table of which file
holds what, the frozen `--font-size-*` cross-boundary contract with an explicit
warning not to "simplify" the test back into a Go-only assertion, why SPA-only
sizes stay outside the ramp naming, and why token values are chosen to be
value-preserving rather than round.

## External / User-Facing Documentation

- [x] ~~docs/data-entry.md~~ (N/A: no user-facing surface. This PR adds no
config key, CLI flag, API field or metamodel construct — a project author cannot
observe it. The one externally-visible artifact, the `_rela.css` served to
custom apps, is deliberately UNCHANGED and pinned by
`TestFrozenTypographyContractMatchesSPA`.)
- [x] ~~docs/metamodel.md, docs/cli-reference.md, README.md~~ (N/A: no
metamodel, CLI or project-level change.)

Note: PR 2 (TKT-5V8704) *does* add a user-facing `span:` config key and will
carry the corresponding `docs/data-entry.md` update.

## Verification

- [x] Documentation matches the implemented behaviour
- [x] Examples in docs actually work

The claims in `frontend/CLAUDE.md` were verified rather than asserted: the
"reads both files off disk" behaviour was negative-tested in both directions,
and the "value-preserving" claim was checked by a script resolving all 197
migrated declarations against `develop` (0 changed).
