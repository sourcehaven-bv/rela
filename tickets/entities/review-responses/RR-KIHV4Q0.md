---
id: RR-KIHV4Q0
type: review-response
title: Writes for another entity void this entity's reads
finding: writesDone counts writes for every address, so the leaving entity's draining saves void the new entity's first load and force retries.
severity: significant
resolution: 'Completed writes are counted per address; EntityDetail tags each write with the address its view was read for. Unit test: a write to another address does not void a read.'
status: addressed
---
