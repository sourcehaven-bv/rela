---
id: RR-3QK26VQ
type: review-response
title: In-memory run store is per process
finding: Without a cache directory the run store is in-memory, so the scheduler and the server do not see each other's runs and the ceiling differs between them.
severity: minor
resolution: 'Not a production case: the in-memory store is used only without a cache directory, which is a test setup. Every deployment shares one run store (pg or sqlite database, or the KV store in .rela with a file lock). The caveat was dropped from GUIDE-flows.'
status: addressed
---
