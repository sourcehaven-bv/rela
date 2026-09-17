---
id: RR-Y4GMZS
type: review-response
title: Self-test ran after the guard, so a spuriously-passing guard reported green first
finding: '.github/workflows/ci.yml: the "Compile build-tag-gated files" step ran before "Test the tagged-build guard". A parser bug that made the guard pass everything would report green on the guard step before the test that would catch it ever ran.'
severity: minor
resolution: Swapped the order so the self-test gates the guard, with a comment stating why.
status: addressed
---
