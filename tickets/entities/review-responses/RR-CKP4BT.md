---
id: RR-CKP4BT
type: review-response
title: Postgres backfill guard does not catch \u0000 and aborts on invalid JSON
finding: '`created_by NOT LIKE ''%\u0000%''` does not do what the plan says. Backslash is LIKE''s default escape character, so the pattern matches the text "u0000", not the six-character escape. Go''s json.Marshal writes a NUL as \u0000, jsonb rejects it, and the `::jsonb` cast then aborts migration 0034 and blocks the upgrade. AC 4 also claims that invalid JSON keeps '''' on postgres. `::jsonb` raises on invalid input, and pg_input_is_valid does not exist before PG16, while the test fleet runs PG15. Fix: guard with `strpos(created_by, ''\u0000'') = 0` (standard_conforming_strings is on), and wrap the parse in a plpgsql function with an EXCEPTION block that returns '''' (or drop the invalid-JSON claim if writes can never produce it). Seed both kinds of row in TestMigration0034_ChannelCreatorBackfill.'
severity: significant
resolution: '0034 backfills via a temporary plpgsql function (0024 pattern): strpos(t,''\u0000'')>0 returns '''', else t::jsonb->>''user'' inside an EXCEPTION block returning ''''. Test seeds a NUL-escape row and an invalid-JSON row.'
status: addressed
---
