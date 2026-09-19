---
id: RR-TVA0Y3
type: review-response
title: Multi-line detection reads the trimmed label, misclassifying edge-newline comments
finding: '`toDOM` computed `const label = body.trim()` and then tested `label.includes(''\n'')` to choose the block class. Trimming removes edge newlines first, so `<!--\nfoo\n-->` — a comment opened on its own line, the most common template shape — was classified inline. It rendered as a cramped chip and never received the `white-space: pre-wrap` rule that preserves interior newlines.'
severity: significant
resolution: 'Changed the test to read the untrimmed `body`. Added two tests: one asserting `<!--\nfoo\n-->` gets `rela-comment-block`, and a negative one asserting a genuinely single-line comment does NOT (without which the fix could simply classify everything as block). Mutation-verified: reverting to `label.includes` fails the first test and passes on the fix. Added a round-trip test for the same shape, since changing how it renders must not change what is stored.'
status: addressed
---

The existing block test used `<!-- For each option:\n- **Pros**: good\n-->`,
which has an INTERIOR newline and so survived trimming. That is why the bug
passed review of my own tests: the fixture happened to sit on the safe side of
the boundary. The new test targets the boundary directly.
