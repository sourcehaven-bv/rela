---
id: RR-MTPE2X
type: review-response
title: 'Desktop is sqlite: private piles would ship inside rela.db'
finding: Desktop builds with the sqlite tag and its state.KV lives in rela.db, so piles would travel in a shipped database; import-fs copies .rela state keys too.
severity: minor
resolution: 'Plan: piles are about the person, not the content. On fs and sqlite, kvpiles runs over a NODE-LOCAL file KV in the cache dir, never rela.db; there is no sqlitepiles and no new sqlite table. fsimport classifies the piles key as personal state and does not copy it.'
status: addressed
---
