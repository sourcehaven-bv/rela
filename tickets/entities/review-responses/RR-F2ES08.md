---
id: RR-F2ES08
type: review-response
title: Duplicated and garbled comment in the tail-cost case
finding: 'internal/channels/filechannels/reads_test.go:155-159: "The doubling tail read and the head lookup together stay within three times the run''s segment." appears twice, with a sentence spliced in between, and line 157 is longer than the others. Keep one copy.'
severity: nit
resolution: ADDRESSED kept one copy of the sentence and restored the dropped clause about the previous segment.
status: addressed
---
