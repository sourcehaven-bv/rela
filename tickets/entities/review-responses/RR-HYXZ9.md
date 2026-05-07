---
id: RR-HYXZ9
type: review-response
title: Test corpus for round-trip property test must be named and pinned
finding: AC2 and AC12 reference 'a corpus' without specifying. Use the actual markdown bodies of in-tree tickets/* entities — they exist, are version-controlled, exercise headings/lists/code/tables/strikethrough/links naturally, and grow as the project does.
severity: minor
resolution: 'Corpus pinned: markdown bodies of every entity under tickets/entities/ plus a small synthetic corpus for edge cases (raw HTML, autolinks, images, hard breaks, nested emphasis). AC13.'
status: addressed
---

# Finding

Plan AC2: "round-trip corpus produces idempotent fixed point." Plan AC12:
"parse(s) == parse(render(parse(s))) for a corpus."

What corpus?

# Resolution

Use a concrete, version-controlled corpus:

- Markdown bodies of every entity under `tickets/entities/`.
These already exist, exercise GFM features naturally (headings, lists, code
blocks, tables, links, strikethrough), and grow as the project grows.
- Plus a small synthetic corpus of edge cases not represented
in real tickets (raw HTML, autolinks, images, hard breaks, nested emphasis).

Add a test that loads these, parses each, renders, re-parses, and asserts AST
equality on the second-parse output.

This makes AC2/AC12 reproducible and reviewable.
