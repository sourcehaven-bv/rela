---
id: RR-7KUDK
type: review-response
title: storage.FS is too wide for the second handle
finding: 'Plan leaves the raw second handle as full storage.FS — which exports ReadFile/WriteFile/Open. Combined with finding #3, loaded gun: anyone adding a feature reaches for s.fs.ReadFile(path) (idiomatic, right there) and silently bypasses decryption — the exact bug the refactor exists to prevent.'
severity: significant
resolution: Plan introduces a narrower DirFS interface (ReadDir, Stat, Walk, Remove) for the raw handle. Structurally omits ReadFile/WriteFile/Open so the compiler prevents any future 'easy' raw-read bypass. AC#3 asserts fsstore.FSStore.dirs does not expose byte I/O.
status: addressed
---
