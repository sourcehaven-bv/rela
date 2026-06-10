---
id: BUG-LSBFD1
type: bug
title: Entity markdown with a line over 64KB is writable but unreadable
description: 'splitFrontmatter (duplicated in internal/markdown and internal/store/fsstore) used bufio.Scanner, which caps a single line at bufio.MaxScanTokenSize (64KB) and returns bufio.ErrTooLong past it. The write path does not use the scanner, so the store happily wrote an entity whose body or a property had a >64KB line (a base64 data: URI, a minified blob, a pasted log) but then failed to read it back: GetEntity returned an I/O error and ListEntities/index rebuild aborted on it.'
priority: high
why1: splitFrontmatter scanned lines with bufio.Scanner, whose default token cap is 64KB; a longer line made scanner.Err() return bufio.ErrTooLong, failing the parse.
why2: The read path used a size-capped line scanner while the write path used plain string joins, so writes and reads had asymmetric limits — a file could be produced that could never be re-read.
why3: The frontmatter split was hand-rolled and duplicated in two packages, so the cap existed in two places and neither had a long-line test.
why4: There was no round-trip property test (write an entity, read it back) covering large single-line content, so the asymmetry went unnoticed.
why5: Low-level .md framing logic had no single owner; duplicated across markdown and fsstore, it accumulated a latent cap with no shared test surface.
prevention: 'Extracted the split into a dependency-free leaf package internal/frontmatter (returns only strings, satisfies the no-leak rule) shared by both markdown and fsstore; it splits on newlines with no per-line cap. Added unit + fuzz tests in the leaf package (incl. >64KB cases) and an fsstore-level write-then-read-back regression that fails without the fix (bufio.Scanner: token too long).'
status: done
---

## Bug

Found in the 2026-06-09 backend review (C4).

`splitFrontmatter` — hand-rolled and **duplicated** in
`internal/markdown/parser.go` and `internal/store/fsstore/markdown.go` — used
`bufio.Scanner`, whose default token cap is `bufio.MaxScanTokenSize` (64 KB). A
single line ≥ 64 KB makes `scanner.Err()` return `bufio.ErrTooLong`, so the
parse fails.

The asymmetry is the trap: the **write** path (`FormatDocumentOrdered`) uses
plain string joins with no cap, so the store writes such an entity fine, but
**read** (`GetEntity` → `parseDocument` → `splitFrontmatter`) then errors.
`ListEntities` and the search-index rebuild abort on it too. Realistic triggers:
a base64 `data:` image URI, a minified JSON/CSV blob, a long table row, a pasted
log line.

## Fix (PR pending)

Extracted the frontmatter split into a new dependency-free leaf package,
**`internal/frontmatter`**, exposing `Split(content) (frontmatter, body
string)`. It returns only strings (no YAML, no domain types — satisfies the
CLAUDE.md "don't leak parsing types" rule), so both `internal/markdown` and
`internal/store/fsstore` (which deliberately doesn't import `internal/markdown`)
share one implementation. The split is on `"\n"` with no per-line cap;
CRLF-tolerant, preserving the scanner's previous observable semantics.

Both hand-rolled `splitFrontmatter` copies are deleted. arch-lint updated to
declare the leaf component and allow `markdown`/`fsstore` → `frontmatter`.

## Tests

- `internal/frontmatter`: `TestSplit` (frontmatter/body, none, empty, only-frontmatter, CRLF), `TestSplit_LongLineExceeds64KB`, `TestSplit_LongLineInFrontmatter`, and `FuzzSplit` (moved from markdown).
- `internal/store/fsstore`: `TestLongLine_WriteThenReadBack` — writes an entity with a 256 KB body line and reads it back (same instance + after reopen). Verified it **fails without the fix** (`bufio.Scanner: token too long`).
