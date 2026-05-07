---
id: RR-NOSDE
type: review-response
title: Round-trip golden AC11 unmatchable as written — refactor deliberately changes link output
finding: AC11 says 'renderer bytes match pre-refactor output for existing test fixtures'. But the whole point of the refactor is to preserve link URLs that were previously dropped. So pre-refactor bytes for any link-containing fixture WILL differ. Restrict AC11 to inputs that don't exercise the new preservation, or rewrite as 'output equivalent up to round-trip closure'.
severity: minor
resolution: AC14 narrows the golden-bytes comparison to fixtures without links/images/raw HTML. AC13 (corpus round-trip property) is the primary correctness anchor and holds regardless of pre/post bytes diff.
status: addressed
---

# Finding

Plan AC11: "Renderer-emitted bytes match the pre-refactor output for the
existing test fixtures (golden)."

The refactor deliberately changes output for inputs containing links, raw HTML,
images, and autolinks — they now round-trip instead of being dropped. So
pre-refactor bytes will not match post-refactor bytes for those inputs.

# Resolution

Reframe AC11:

- **Old behavior preserved for non-link content**: existing
fixtures that don't contain links/images/raw HTML round-trip to identical bytes.
Use the existing fixtures from `markdown_test.go` (most are basic
paragraphs/headings/lists).
- **New behavior for link-bearing content**: a separate AC
asserts new fixtures with links produce link-preserving output.

This splits the "no-regression" claim from the "new feature" claim, which is
what AC11 was trying to do anyway.

Update AC2 ("round-trip corpus produces idempotent fixed point") to be the
primary correctness anchor — that one is true regardless of whether output bytes
change vs. pre-refactor.
