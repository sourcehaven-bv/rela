---
id: RR-1ND28L
type: review-response
title: Edited a generated doc instead of its docs-project source
finding: docs/data-entry.md is generated from docs-project/entities/guides/GUIDE-data-entry.md and carries a 'Do not edit directly' header I did not read before editing. The next `just docs` run silently deleted the new section. Caught by the docs-check gate in `just ci`, which compares the generated tree against its source — but only after the content had already been committed to the wrong file.
severity: minor
resolution: Moved the section into the docs-project entity and regenerated. `just docs-check` passes.
status: addressed
---

## Finding

34 files under `docs/` open with:

```markdown
<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->
```

I added the body-editor documentation to `docs/data-entry.md` without reading
the first line of the file. Running `just docs` deleted it, which is the correct
behaviour of a generated tree.

## Why it was caught late

`just ci` runs `docs-check`, which is what surfaced it. Worth noting how nearly
it was missed: the backgrounded `just ci` reported **exit code 0** while
`docs-check` had failed inside it. The failure was only visible because a
monitor was watching the output stream for `recipe ... failed`.

That exit code is not to be trusted for this recipe. Read the output.

## Resolution

Section moved to `docs-project/entities/guides/GUIDE-data-entry.md`, regenerated
with `just docs`, and both trees committed together. `just docs-check` exits 0.
