---
id: BUGA-KUGLRL
type: bug-analysis-checklist
title: 'Analysis: sqlite short-needle search folds case for ASCII only'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally. The new storetest case
`RunFieldScopeSearchTests/BackendFoldsLikeGo` fails on sqlitestore at a8fbdbbe4
for both its needles: "é" (short, `LIKE`) and "ԩab" (long, FTS5 trigram).
- [x] Minimal reproduction steps documented. Two tickets with title "CAFÉ";
the first also has `code: café`, and `code` is hidden. A search for "é" finds
only the first, through the hidden `code`, and the Go judgement then admits it
because the title "CAFÉ" contains "é" under Go's fold. Whether the hit appears
depends on the hidden value. The same holds for title "ԨAB" and needle "ԩab".
- [x] Environment/conditions noted. sqlite build only, under a `visible:`
policy. pgstore matches Go-lowered `search_text` and was not affected.

## Root Cause

- [x] Immediate cause identified (why1): sqlitestore matched needles under 3
runes with `LIKE`, which folds ASCII only, and longer ones with an FTS5 trigram
index that folds case with its own Unicode tables. `search.MatchTextFields`
judges fields with Go's `strings.ToLower`.
- [x] Contributing factors found (why2-3): the index text was built in SQL by
triggers, so it could not use Go's fold. FTS5's fold differs from Go's in both
directions (a probe found U+0528, U+0130 and U+13A0 folded only by Go, and
U+03C2 and U+00B5 only by FTS5).
- [x] Systemic cause explored (why4-5): the rule that SQL and Go must fold
alike was written for pgstore's `lower()` and checked only there. The field
scope conformance cases used Latin letters that both folds agree on.

## Fix Planning

- [x] Fix approach determined. The store writes `entities.search_text` in Go
with `search.IndexText` (shared with pgstore), the FTS5 index uses a
`case_sensitive 1` trigram tokenizer over it, and short needles use `instr`.
The v19 rung adds the column and drops the old index; `sqlitestore.New` fills
the column for existing rows.
- [x] Regression test planned. `BackendFoldsLikeGo` in
`storetest.RunFieldScopeSearchTests` pins a short and a long non-ASCII needle on
every backend; `TestSearch_MigrationRebuildsIndex` gains a v18 case.
- [x] Related areas checked for similar issues. pgstore uses Go-lowered
`search_text` with `LIKE` (case-sensitive in PostgreSQL). bleve and the linear
search judge in Go. Title ranking in sqlite still uses `lower()`, which affects
order only, not which rows match.
