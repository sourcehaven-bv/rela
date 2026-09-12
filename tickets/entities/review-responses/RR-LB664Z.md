---
id: RR-LB664Z
type: review-response
title: 'CreateRelation allocated a lineage id before the uniqueness check, burning one on every duplicate'
finding: 'internal/store/sqlitestore/relation.go: nextRelRecordID bumped rel_record_seq before the INSERT. Outside a Tx that UPDATE autocommits, so when the INSERT hit the primary-key conflict the counter had already advanced permanently. ErrConflict on a duplicate triple is an ordinary outcome that automations retry on, not an exceptional one, so every retry leaked an id. The ordering was the defect: allocation should be a consequence of a successful insert, not a precondition for attempting one.'
severity: critical
resolution: 'The INSERT now reads the counter inline via (SELECT next FROM rel_record_seq WHERE id = 1) and a separate bumpRelRecordSeq consumes the id only once that INSERT has succeeded. A rejected duplicate therefore consumes nothing. Verified against modernc/sqlite that a PK conflict on this shape leaves the counter untouched while distinct rows still get distinct ids. Pinned by TestDuplicateRelationDoesNotBurnALineageID, which was confirmed non-vacuous by temporarily restoring the old order (it fails).'
status: addressed
---
