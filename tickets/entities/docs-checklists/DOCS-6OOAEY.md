---
id: DOCS-6OOAEY
type: docs-checklist
title: 'Docs: Named query scopes declared per entity type in schema.yaml'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported types and functions
- [x] Rationale recorded where a reader would otherwise ask "why"
- [x] Stale comments corrected

`internal/scopes` carries a package doc covering the boundary (why a separate
package), the UX-not-access-control framing, and identity. `scopedread.go`'s doc
records the two historical fail-open bugs, the withheld-vs-empty split, and the
one sanctioned duplicate. `resolvedQueryScope` documents why the four pieces
travel together, naming the bug that happened when they did not.

One stale comment was corrected: `QueryScopeParam`'s doc asserted in the present
tense that the SPA attached the parameter, describing code that did not exist.
It now states that the client is a required participant, and why that is worth
saying — a `query_scope:` validates and derives an index whether or not anything
sends it.

## Project Documentation

- [x] `docs/metamodel.md` — the `query_scopes:` declaration
- [x] `docs/data-entry.md` — `query_scope:` on views and `?query_scope=`
- [x] ~~`docs/cli-reference.md`~~ (N/A: AC6 pins that the CLI deliberately does
NOT apply scopes, so there is no flag or behaviour to document)
- [x] ~~`README.md`~~ (N/A: not a project-level change)

Written in `docs-project/entities/guides/` and regenerated with
`scripts/generate-docs.sh`, per the repo's generated-docs rule.

The metamodel section sits beside "Content States and Worlds" with a table
contrasting the two, because the adjacency invites confusing them — a world
ranks faces and never changes the row count, a scope includes or excludes and is
exactly a change to the row count. It also states the two things the ticket
asked to be said once: that a scope is presentation rather than access control
(the two read alike in YAML, so `mijn:` looks like a restriction and is not),
and which surfaces apply scopes, with the reason the analysis tools do not.

## External Documentation

- [x] ~~Release notes / changelog~~ (N/A: this repo has no changelog; the
commit messages and ticket carry the history)
- [x] ~~API reference~~ (N/A: `?query_scope=` is documented in the data-entry
guide alongside `?world=`, which is where the API surface is described)
