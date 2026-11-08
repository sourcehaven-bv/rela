---
id: RR-SYGLCK1
type: review-response
title: Temporary output dir survives a missing Close
finding: Cleanup runs only from Services.Close.
severity: minor
reason: Close is required by the NewInMemory contract (it also stops background workers); the dir is mode 0700 under TMPDIR.
status: wont-fix
---
