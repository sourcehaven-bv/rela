---
id: RR-73AGM0
type: review-response
title: content_hash in rela.history leaks hidden values
finding: The stored content_hash covers the unredacted snapshot, so a script can brute-force a low-entropy hidden field or detect hidden changes. HTTP never serves it.
severity: critical
resolution: Removed content_hash from the Lua timeline; test asserts it is absent; docs point sync scripts at the version number.
status: addressed
---
