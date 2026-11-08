---
id: RR-D0R3LJS
type: review-response
title: In-memory FS check copy-pasted at eight sites
finding: Each lock-file site repeated fs.(*storage.MemFS), so the convention was easy to miss.
severity: significant
resolution: Added one predicate inMemoryFS in memrecipe.go with the rationale in its doc; all appbuild sites use it. SafeFS has its own allow-list check in the storage package.
status: addressed
---
