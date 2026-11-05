---
id: RR-9QVT0LM
type: review-response
title: Fence exemption hides real merges inside code blocks
finding: Git moves a fence line both sides share above the start marker. A real conflict inside a code block was then read as body text, left out of the conflict list, and the next save stored the markers for good.
severity: significant
resolution: Fenced code no longer exempts a block. A complete block at column 0 is a conflict anywhere. A document that shows an example indents its marker lines; fsstore refuses to write a document that would read as conflicted. Pinned in TestConflictBlocks (merge inside a code block) and TestWrite_ConflictBlockBodyStaysReadable.
status: addressed
---
