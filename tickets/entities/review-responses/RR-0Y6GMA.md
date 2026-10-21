---
id: RR-0Y6GMA
type: review-response
title: Saving the in-memory entries with the observed mtimes is unsound when memory diverges from disk without a directory change
finding: 'The plan saves the observed (open-time) mtimes but still writes the in-memory entries at close. That is safe only if every in-memory change to a directory came with an on-disk change to that directory. The watcher breaks this: reconcileEntityPath takes id and type from the frontmatter, not from the filename and directory, so a file plans/A.md with id B or type task is held under the wrong key or type; and any read error (a chmod that makes a file unreadable fires an event but leaves the directory mtime unchanged) is treated as a removal by handleEntityRemoval. The saved directory then has a matching mtime and a wrong entry list, which is the same "entity not found until the cache is deleted" failure. Grouping entries by directory at save time also needs a directory per entry, which entityMeta does not carry; deriving it from Type through the plural map files watcher-sourced entries under the wrong directory. If the cache is kept, persist per directory the listing captured at open (the entries the scan or cache produced, before any mutation), not the live index; or keep a per-directory dirty flag set by every index mutation and save dirty directories with a zero mtime. State in the plan which one, and add a test where the watcher changes memory without a directory mtime change.'
severity: significant
resolution: 'Resolved by removing the cache: nothing saves the in-memory index any more.'
status: addressed
---
