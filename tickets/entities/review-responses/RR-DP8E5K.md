---
id: RR-DP8E5K
type: review-response
title: pg rename hook can fail on a concurrent add
finding: 'internal/piles/pgpiles/pg.go RenameEntity: an add of the new id racing the hook makes the UPDATE fail with 23505 after the add commits; the hook errors and every pile keeps the old id.'
severity: minor
resolution: RenameEntity copies old rows to the new id keeping seq and face with INSERT ... ON CONFLICT DO NOTHING, then deletes the old rows; a concurrent add no longer fails the hook.
status: addressed
---
