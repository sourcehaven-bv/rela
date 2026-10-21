---
id: RR-URQDJW
type: review-response
title: Watcher gate checks the type but not the id, and removal is not gated at all
finding: 'internal/store/fsstore/watcher.go:219. The new check requires the directory type to match the frontmatter type but still takes the id from frontmatter. A copied file entities/requirements/REQ-9.md that still says id: REQ-1 is indexed under REQ-1: search gets REQ-9''s content for REQ-1, and reads still resolve REQ-1.md. This is the same "file indexed somewhere other than where reads look" defect. On the removal side, handleEntityRemoval keys by the filename stem alone, so deleting a stray entities/riskregister/REQ-1.md drops the real REQ-1 from the index while requirements/REQ-1.md still exists. Fix: also require the parsed id and face to equal entityIdentityFromPath''s; in handleEntityRemoval, act only when the removed path equals absPath(entityFileKey(meta.Type, stem)).'
severity: minor
resolution: The watcher requires the frontmatter id to match the filename stem, and a remove event drops an entry only when the removed path is that entry's own path. Tests fail without the fix.
status: addressed
---
