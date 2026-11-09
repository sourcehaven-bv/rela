---
id: RR-Y197PJC
type: review-response
title: SafeFS uses a MemFS deny-list instead of an OsFS allow-list
finding: Any wrapper other than a bare MemFS (SafeFS(SafeFS(MemFS)) or ErrorFS) fell through to os calls and wrote to disk.
severity: significant
resolution: SafeFS.WriteFile now takes the os temp+rename path only when the inner FS is *OsFS; everything else delegates to the wrapped FS. Table test covers memfs and safefs-over-memfs and errorfs-over-memfs; a nested-over-disk test checks no temp file is left.
status: addressed
---
