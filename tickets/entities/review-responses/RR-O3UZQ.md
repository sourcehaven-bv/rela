---
id: RR-O3UZQ
type: review-response
title: AC#4 byte-for-byte identical is unverifiable without snapshot
finding: 'AC4: ''Encryption off: on-disk bytes byte-for-byte identical to pre-refactor cleartext output.'' Tests passing doesn''t prove byte equality, and there''s no captured snapshot to diff against. After refactor, you''d be diffing new output against new output.'
severity: nit
resolution: 'AC#4 replaced with AC#7: ''fsstore_test.go golden-file assertions unchanged.'' Cheaper than a captured tarball, equally meaningful, actually verifiable.'
status: addressed
---
