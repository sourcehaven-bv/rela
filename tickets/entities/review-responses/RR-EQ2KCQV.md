---
id: RR-EQ2KCQV
type: review-response
title: Off-screen instances still move their bases
finding: A replaced instance keeps receiving server responses and moves its bases while its apply callbacks do nothing.
severity: minor
resolution: 'Documented on makeContentAutoSave: a replaced instance must not take new edits, and returning to its row builds a fresh one.'
status: addressed
---
