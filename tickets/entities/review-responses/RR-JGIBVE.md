---
id: RR-JGIBVE
type: review-response
title: SetSchemas refusal tells the operator to restart, and a restart hides every entity of that type
finding: 'internal/store/fsstore/schemas.go:59 returns "restart rela mcp to apply the new plural for type %q". After a restart, New scans with the new plural, the old directory matches no type and is skipped (index.go scanEntityDirs), so every existing entity of the type vanishes from reads. New writes then go to the new directory, so a sequential id generator can reissue ids that still exist in the old one, and merging the directories later collides. The advice leads straight to apparent data loss. The same silent orphaning happens on rela-server when a Configure save changes entities.*.plural (allowlisted) and the next generation reopens the store. Fix: change the message to say what to do: "type %q moved from entities/%s to entities/%s; move the files there (or use rela rename entity) and restart". Consider making New warn when a non-empty directory under entities/ matches no declared plural.'
severity: significant
resolution: The refusal now says to rename entities/<old> to entities/<new> and restart. The pre-existing loss on restart or a Configure save without the rename is filed as TKT-NUMUJD.
status: addressed
---
