---
id: RR-KH3O0
type: review-response
title: persistedIndex field names use 'dir' language but values are now keys
finding: JSON fields entities_dir_mtime/relations_dir_mtime still say 'dir' but the values are now keys. Future readers will waste time wondering what the distinction is.
severity: minor
reason: Renaming would break forward-compat with existing .rela/fsstore-index.json files in the wild (cache would rebuild from scratch on first load). The name is slightly misleading but the mtimes are unchanged (they come from rooted.Stat(key) which statues the absolute path). Acceptable cosmetic drift; not worth a cache rebuild. Comment would make future readers pause — revisit if/when we bump a cache format version for other reasons.
status: wont-fix
---
