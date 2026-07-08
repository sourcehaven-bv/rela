# Name the reserved-key predicates for entity/relation frontmatter

## What & why

gocleaner's `02-diverging-literal-allowlists` detector flagged two implicit
concepts guarded by copy-pasted literal sets (findings `2efd69b41dfe`,
score 79.0 — rank 4 in the attention ranking — and its group-mate
`394d3bcd3f97`):

- `{id, type}` checked at 8 sites across `conflict`, `dataentryconfig`,
  `importer`, `fsstore`, `storetest`, `templating` — with one divergent
  variant `{id, type, _template_relations}` in `templating`.
- `{from, relation, to}` duplicated identically at 3 sites
  (`conflict`, `fsstore`, `templating`).

These are the *reserved identity keys* of an entity/relation document or
row: they map to `entity.Entity` / `entity.Relation` struct fields and
must never land in `Properties`, and validators must accept them as
filter targets even though they are not schema-declared properties.
Nothing kept the copies aligned; a new parser or validator had to
rediscover the sets by grepping.

**Judgment (intentional-vs-drift), recorded on the finding:** the one
divergent variant is intentional policy, not drift. Entity *template*
files reserve an additional templating-only frontmatter key
`_template_relations` (default relations, read by
`docTemplateRelations` / `extractTemplateRelations`). That is a
template-format concept owned by `internal/templating`, not an entity
concept — so it stays local, as a named constant composed with the
shared predicate. All other sites agree on the same sets; no set was
changed anywhere, so the refactor is behavior-preserving by
construction.

## Approach

The recipe's **intended** option (`name-the-predicate`): reify each
concept as one named predicate in the package that owns it.

- Owner: `internal/entity` — the pure domain leaf where the
  identity-vs-property distinction is defined, and an arch-lint
  `commonComponents` member every call site may already import (no
  boundary changes needed; `just arch-lint` stays green).
- New: `entity.IsReservedEntityKey` (`id`, `type`) and
  `entity.IsReservedRelationKey` (`from`, `relation`, `to`), with doc
  comments naming the concept and the composition rule for
  format-specific extra keys.
- The templating policy is named: `templateRelationsKey` constant
  replaces four scattered `"_template_relations"` literals; the entity
  template property filter reads
  `entity.IsReservedEntityKey(k) || k == templateRelationsKey`.

Beyond the finding's evidence, two more same-concept sites the detector
missed were routed through the predicate as well
(`internal/dataentryconfig/validate.go:456` and `:988`, list/kanban
filter validation — same `{id, type}` check as the flagged
`validate_feeds.go:114`).

## Commits

1. `entity: add reserved-key predicates` — `internal/entity/reserved.go`
   plus table-driven tests (case sensitivity, cross-set confusion:
   `type` is an entity key but *not* a relation frontmatter key).
2. `route reserved-key checks through entity predicates` — the 10
   entity-key sites and 2 relation-key sites outside templating
   (`conflict/parse.go`, `dataentryconfig/validate{,_feeds}.go`,
   `importer/importer.go`, `store/fsstore/{markdown,watcher}.go`,
   `store/storetest/fuzz.go`).
3. `templating: name the template-relations key` — the
   `templateRelationsKey` constant and the remaining three sites
   (`fstemplater.go`, `fsloader.go`).

## Verification

- `go build ./...` clean; full `go test ./...` green (all packages).
- `golangci-lint run ./...` — 0 issues.
- `go-arch-lint check` — no warnings (entity is a common component; the
  two new imports of it in `dataentryconfig` and `templating` are
  allowed).
- gocleaner re-run (analyze + detect) on the modified tree:
  - before: 35 findings, 31 literal-sets; both findings present
    (`{_template_relations, id, type}` at 8 sites / 2 variants;
    `{from, relation, to}` at 3 sites).
  - after: 33 findings, 22 literal-sets; **both findings gone** — the
    only remaining copies of the sets are inside the two predicates.
- `git diff --stat`: 12 files, ~54 changed lines excluding tests —
  nothing unrelated touched.

## Follow-ups deliberately not done

- `{y, yes}`, `{http, https}` and `{description, priority, status,
  title}` literal-set findings — different concepts (CLI confirmation,
  URL schemes, output ordering), each deserving its own small change.
- `internal/dataentryconfig/validate.go:440` and
  `internal/metamodel/loader.go:426` check `{id, modified}` for *sort*
  targets — a related but distinct concept ("sortable pseudo-columns",
  includes `modified`, excludes `type`); left as-is rather than force it
  under the entity-key predicate.
- Writer-side key literals (e.g. `conflict/resolve.go` building
  `{"id": ..., "type": ...}` frontmatter) still name the keys directly;
  exporting key-name constants was judged over-engineering for now.
