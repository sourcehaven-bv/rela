---
id: RR-9NSDNW
type: review-response
title: kvpiles loses writes across processes on one project
finding: internal/piles/kvpiles/kv.go only uses an in-process mutex, but rela-desktop, rela mcp (stdio), CLI Lua and rela scheduler each build their own Store over the same .rela piles.json. Two processes doing load-modify-write interleave and one silently drops the other's add; the pile cap and delete hooks race the same way.
severity: significant
resolution: kvpiles.New requires a Locker; appbuild supplies an OS file lock (piles.lock beside piles.json, unix + windows, reusing the sqlitedb approach) and only an in-memory FS gets the process-private locker. TestFileLocker_TwoStoresLoseNothing runs two stores over one file under -race and fails without the file lock.
status: addressed
---
