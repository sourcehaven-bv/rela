---
id: RR-NP9T1J
type: review-response
title: Docs plan edits generated files; the source is docs-project/ and just ci fails on the diff
finding: The plan's Documentation Planning section names docs/lua-scripting.md (rows :342, :345, :425) and docs/content-states.md as the files to edit. Both are BUILD OUTPUTS. justfile:423-425 runs scripts/generate-docs.sh, which sets DOCS_PROJECT=$ROOT/docs-project and OUTPUT=$ROOT/docs (generate-docs.sh:14-18,35) and generates docs/ from the docs-project entity graph. justfile:509-514 defines docs-check, part of `just ci` at :517, which runs `git diff --exit-code docs/ README.md docs-project/` and fails if they differ. So an edit to docs/lua-scripting.md is reverted by the next `just docs` and fails CI. The real sources are docs-project/entities/guides/GUIDE-lua-scripting.md (mutation table :348/:351, admin table :431) and docs-project/entities/guides/GUIDE-content-states.md, both verified to exist. Worse, the naive fix — editing both and committing — papers over the generation step and leaves the next `just docs` to revert it silently.
severity: critical
resolution: 'Accepted and verified. Documentation Planning rewritten to target docs-project/entities/guides/GUIDE-lua-scripting.md and GUIDE-content-states.md, with an explicit ''run just docs and commit the regenerated docs/'' step. The generation chain and the docs-check CI gate are now quoted in the plan so the next reader does not repeat the mistake. Also recorded the reviewer''s sub-point: the create_relation content? phantom exists only in the Go doc comment, not in the user-facing signature table, so that docs item was overstated and is now scoped to a code-comment fix.'
status: addressed
---

## Finding

The plan's Documentation Planning section names two files to edit:

- `docs/lua-scripting.md` — signature table rows at `:342`, `:345`, `:425`
- `docs/content-states.md`

**Both are build outputs, not sources.**

`justfile:423-425` runs `scripts/generate-docs.sh`, which sets
(`generate-docs.sh:14-18`):

```sh
DOCS_PROJECT="$ROOT/docs-project"
OUTPUT="$ROOT/docs"
```

and at `:35` generates `docs/` from the `docs-project/` entity graph via
`scripts/generate-docs.lua`.

`justfile:509-514` then defines `docs-check`, which is part of `just ci`
(`:517`):

```
git diff --exit-code docs/ README.md docs-project/ || \
    (echo "ERROR: docs/, README.md or docs-project/ is out of date." && exit 1)
```

So editing `docs/lua-scripting.md` directly produces a guaranteed CI failure,
and the edit is reverted by the next `just docs` run.

## The real sources

Both verified present:

- `docs-project/entities/guides/GUIDE-lua-scripting.md` — the mutation-table
rows are at `:348`/`:351`, the admin table at `:431`
- `docs-project/entities/guides/GUIDE-content-states.md`

## Why this is worth a critical rating

The cost is not the mistake but *when* it surfaces. The docs item is the last
thing done on the ticket, so this fails at the end of the work, after the code
is finished and reviewed — the most expensive moment to discover a wrong target.

And the obvious recovery is wrong: editing both the source and the generated
file and committing them papers over the generation step, leaving the next `just
docs` to silently revert half of it. The correct sequence is edit the
`GUIDE-*.md` sources, run `just docs`, commit the regenerated `docs/`.

## Fix

Rewrite both Documentation Planning items to name the
`docs-project/entities/guides/GUIDE-*.md` files, and add an explicit step: **run
`just docs` and commit the regenerated `docs/`.**

Worth noting for the `create_relation` doc row specifically: the generated
`docs/lua-scripting.md:345` and its source `GUIDE-lua-scripting.md:351` both
already read `rela.create_relation(from, type, to)` — three arguments, no
`content?`. The phantom argument exists only in the Go doc comment at
`internal/lua/runtime.go:1882`, so that part is a code-comment fix with no
user-facing doc row to correct. The plan overstates that item.
