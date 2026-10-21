---
id: TKT-H63QDR
type: ticket
title: fsstore temp-file cleanup can delete another process's in-flight write
kind: chore
priority: medium
effort: s
started: "2026-10-21"
completed: "2026-10-21"
status: done
description: cleanupTempFiles in fsstore.New deletes *.tmp files that another running process is about to rename, so that process's write fails.
---

## Description

`fsstore.New` calls `cleanupTempFiles`, which deletes leftover `*.tmp` files
under the store's directories. `SafeFS.WriteFile` writes to `path + ".tmp"` and
renames it. When another rela process on the same project is between that write
and the rename, the new process deletes its temp file and the other process's
rename fails, so its write fails.

The fixed temp name also means two processes writing the same file at once share
one temp file.

Found in the design review of TKT-33078A (RR-SEPGPF).

## Fix direction

Use a unique temp name per write (for example `path.<random>.tmp` via
`os.CreateTemp` in the same directory), and have `cleanupTempFiles` delete only
temp files older than a grace period (for example one minute), so a live
writer's file is never removed.
