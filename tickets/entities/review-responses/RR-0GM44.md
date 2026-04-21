---
id: RR-0GM44
type: review-response
title: FSKV.Put removed lazy .rela/ creation safety net
finding: Old Put did MkdirAll(Dir(full)) unconditionally, which also created the root directory itself if missing. New version only created parents for nested keys, so top-level keys assumed root exists.
severity: significant
resolution: Resolved together with the filepath.Dir issue above. RootedFS.WriteFile now calls MkdirAll(filepath.Dir(full)) on every write, which creates the root itself when the first top-level key is written. Matches SafeFS.WriteFile semantics. Test TestRootedFS_WriteFile_CreatesParentDirs added.
status: addressed
---
