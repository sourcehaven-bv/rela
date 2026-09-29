---
id: RR-GAAE2J
type: review-response
title: Entry parsing and dedup edge cases
finding: FileRefs did not clean paths, so ./ or // could turn a keyed entry into a legacy one; dedup order was not deterministic; deleting a name held twice removed only one entry.
severity: minor
resolution: FileRefs cleans the path before parsing; SortFileRefs orders by name, key and entry; deleteFile removes every entry with the name. TestStorageKey_CleansEntry and TestFaced_MixedLegacyAndKeyedEntries.
status: addressed
---
