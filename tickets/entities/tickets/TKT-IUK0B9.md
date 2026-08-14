---
id: TKT-IUK0B9
type: ticket
title: Fix malformed YAML frontmatter in AM-feed-field-redaction
kind: chore
priority: medium
effort: xs
status: done
---

## Problem

`tickets/entities/automated-measures/AM-feed-field-redaction.md` (added in
#1314) has unquoted frontmatter scalars containing a colon-space sequence.
YAML reads `visible: -hidden` as a nested mapping, so the whole project fails
to parse:

```
failed to parse frontmatter: yaml: line 4: mapping values are not allowed in this context
```

This is not cosmetic — it aborts entity-ID collection, so **every `rela create`
against the `tickets` project fails**, and `rela list` exits non-zero. The
backfill indexer logs it as a list error on startup.

Two lines are affected:

- line 4 `title:` — contains `visible:-hidden`
- line 5 `description:` — contains `where: clause`

Line 4 errors first; line 5 would surface immediately after.

## Fix

Single-quote both scalars. No content change — the text round-trips identically
(verified via `rela show`).

## Verification

- `rela list --project tickets` exits 0 and reports 2390 entities (previously
  exited 1)
- `rela create ticket --project tickets` succeeds again (this ticket was
  created with it, which was impossible before the fix)
- `rela show AM-feed-field-redaction` renders the title and description
  unchanged

## Note

Two other files were flagged by a naive scan that split on `---` anywhere in
the file: `RR-0EWZQW.md` and `BUG-1VVXHZ.md`. Both are **valid** — they contain
`--- SKIP` / `--- FAIL` mid-line inside quoted scalars, which only confuses a
parser that does not require the closing fence to be line-initial. rela parses
both correctly; no change needed.
