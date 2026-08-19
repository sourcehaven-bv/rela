---
id: DOCS-IY6OI7
type: docs-checklist
title: 'Docs: Aggregate-over-hidden-rows documents (TKT-Y3JVFK)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

Key godoc, each explaining *why* rather than restating the code:

- `metamodel.ACLBypass` — what the flag is FOR (a triage marker for review, not
a permission model), why it is not sliced by verb, and why `create` was
rejected (no elevated `create_entity` exists, so it would name a capability the
system lacks — DEC-O59WM4's "appears to work" failure).
- `ACLBypass.UnmarshalYAML` — why the legacy bool is refused rather than
accepted: a shim on a privilege field resolves toward MORE access if the
spellings drift, and nothing ever forces its removal.
- `lua.newElevatedHandle` — why write methods are ABSENT when the mutator is
nil while a nil READER leaves methods present-and-raising. The asymmetry is the
non-obvious part: nil reader = misconfiguration worth naming; nil mutator on a
render = read-only by construction.
- `lua.WriteDeps.ElevatedManager` / `.ElevatedReader` — that either now
registers `bypass_acl`, and what each controls.
- `dataentry.authorizeElevatedDocument` — why it keys on the ACL
implementation instead of the read gate (the RR-CWWJGW fail-open), and why
NopACL denies here while `authorizeCommand` grants.
- `dataentry.elevationRecorder` — the typed-nil trap it exists to avoid.
- `dataentryconfig.DocumentConfig.Permission` — REWRITTEN: the field's meaning
is now conditional (misinformation guard when gated, confidentiality boundary
when elevated), replacing a rationale that did not hold.
- `dataentryconfig.DocumentConfig.AllowACLBypass` — why writes are refused on a
GET, and that the script is trusted code.
- `documentRenderConfig.Elevated`, `documentService.elevatedDeps` — why
elevation is per-render rather than in the shared deps bundle.
- `migration.ACLBypassEnumMigration` — why the migration is necessary rather
than optional, and why falsy is dropped rather than rewritten.

## Project Documentation

**`docs/*.md` are GENERATED from `docs-project/entities/`** — edit the source
guide, then run `just docs`. I initially edited the generated files, which the
next `just docs` would have silently reverted; DOCS-CFJWKI records the same
mistake on TKT-M1AX6P and prescribes the check that caught it:

```console
$ just docs && git diff --stat docs/
```

Regeneration is now a no-op against the committed docs (verified twice, and the
duplicate-heading markdownlint failure from a bad first port is fixed).

- [x] README updated (if applicable) — N/A: no project-level change.
- [x] CLAUDE.md updated (if new patterns)
  - `internal/dataentry/CLAUDE.md`: the `permission:`-is-an-intent-gate rule
now says "UNLESS the document is elevated", plus the second gate, the read-only
restriction, and the trusted-code note. The existing rule said "don't harden it
into a required field" — leaving that unqualified next to code that DOES
require it for elevated documents would have been actively misleading.
- [x] Help text accurate (if CLI changes) — N/A: no CLI surface changed.
- [x] `docs-project/entities/guides/GUIDE-lua-scripting.md` → `docs/lua-scripting.md`
  - `allow_acl_bypass` enum table (which methods each value grants; ungranted
methods are ABSENT, not failing)
  - what the flag is for (review triage) and the migration note
  - new "Elevated reads in a document render" section
- [x] `docs-project/entities/guides/GUIDE-data-entry.md` → `docs/data-entry.md`
  - `allow_acl_bypass` row in the documents field table
  - the `permission:` paragraph qualified: on an ordinary document it buys
honesty about scope (a report claiming company-wide numbers over a tenth of the
rows leaks nothing and asserts something untrue)
  - new "Elevated documents" section: the use case, the three config rules, the
trusted-code warning
- [x] `docs-project/entities/guides/GUIDE-acl-security.md` → `docs/acl-security.md`
  - new "An elevated document is trusted code" section stating the accepted
risk plainly: `permission:` grants "may read whatever this script reads", not
"may view this report". The aggregation IS the confidentiality boundary and
nothing enforces it, so the mitigation is review — said outright rather than
left for a reader to assume the gate does more than it can (RR-LWD8N3).

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repo has no CHANGELOG.md)
- [x] API docs updated (if applicable) — N/A: no new endpoint or wire-format
change. `allow_acl_bypass` is config, and `/api/v1/_config` already serves
`DocumentConfig` verbatim; the new field appears there automatically, which is
correct (config is not a secret).
