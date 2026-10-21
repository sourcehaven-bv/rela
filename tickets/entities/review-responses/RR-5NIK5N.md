---
id: RR-5NIK5N
type: review-response
title: Sweep test no longer proves real files survive
finding: 'internal/store/fsstore/tempfile_test.go:38-68 replaces the deleted recovery_test.go cases (TestRecovery_Orphaned*, TestRecovery_MultipleTempFiles). Those also asserted that the sweep leaves real entity files alone and that the store still serves REQ-1 afterwards. The new test checks only a stale temp file and a fresh one. A predicate regression that matched .md files would pass. Fix: in each case also write a stale real file (e.g. entities/requirements/REQ-1.md with entity frontmatter, mtime older than TempStaleAfter) and assert it still exists and GetEntity returns it. The analysis test already does this with REQ-5.md.'
severity: minor
resolution: Each sweep case also holds a stale real entity and relation, and the test checks both stay readable through the store.
status: addressed
---
