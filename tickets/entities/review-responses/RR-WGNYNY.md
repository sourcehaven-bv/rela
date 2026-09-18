---
id: RR-WGNYNY
type: review-response
title: docs/data-entry.md is generated; the hand-edit was discarded by `just docs` and failed the docs-check gate
finding: 'I wrote the new operator documentation directly into docs/data-entry.md. That file is GENERATED from docs-project/entities/guides/GUIDE-data-entry.md by scripts/generate-docs.sh, so `just docs` deleted all 80 lines and `just docs-check` failed with "docs/, README.md or docs-project/ is out of date". Caught only by the full `just ci` run, because docs-check is not part of `just lint` or `just test` — every gate I had run up to that point passed. A second wrinkle: the generator rewraps prose, so the source entity must already hold the text in the shape the generator emits, or regeneration oscillates and the gate never goes green.'
severity: significant
resolution: Moved the content into the source entity, in the generator's own wrapping, and verified regeneration is now a fixpoint (`just docs` twice produces no diff) and that `just docs-check` passes. The planning checklist's Documentation Impact named docs/data-entry.md as the target, which was correct about WHERE the docs surface but not about which file is the source — worth remembering that under docs/ the generated files are the majority, not the exception.
status: addressed
---

## Finding

I wrote the new operator documentation directly into `docs/data-entry.md`. That
file is **generated** from `docs-project/entities/guides/GUIDE-data-entry.md` by
`scripts/generate-docs.sh`, so `just docs` deleted all 80 lines and `just
docs-check` failed:

```
ERROR: docs/, README.md or docs-project/ is out of date.
```

Caught only by the full `just ci` run. `docs-check` is not part of `just lint`
or `just test`, so every gate I had run before that point passed — including
`lint-md`, which happily linted the doomed file.

A second wrinkle: the generator **rewraps prose**, so the source entity must
already hold the text in the shape the generator emits. My first two attempts to
align it oscillated, because I was rewording to match instead of taking the
generator's output as canonical.

## Resolution

Content moved into the source entity, in the generator's own wrapping. Verified
that regeneration is a fixpoint — `just docs` twice produces no diff — and that
`just docs-check` passes.

The planning checklist's Documentation Impact named `docs/data-entry.md` as the
target. That was right about *where* the docs surface is and wrong about which
file is the source. Worth remembering that under `docs/` the generated files are
the majority, not the exception, so "which entity produces this?" is the first
question, not an afterthought.
