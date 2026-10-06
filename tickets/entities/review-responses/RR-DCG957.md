---
id: RR-DCG957
type: review-response
title: Line diff used quadratic memory
finding: matchLines stored the whole V array per round, so a large file with many edits could use gigabytes.
severity: significant
resolution: Each round stores only its window, and the merge gives up past maxEditLines (2000) and writes the encoding.
status: addressed
---
