---
id: RR-ZGHOI1
type: review-response
title: rela-docs screenshot capture could seed fixtures into a real database under -tags sqlite
finding: capturer_fs.go was tagged !postgres, so the sqlite build inherited it -- including a doc comment asserting 'a throwaway temp project via appbuild.Discover -- an ephemeral fsstore that never touches real data', which is false under -tags sqlite. capturer_postgres.go exists precisely to prevent this class of contamination. appbuild.Discover resolves through project.Discover, which walks UPWARD from the given directory, so a temp path nested under a real project seeds fixture entities into that project's database via a raw store write bypassing entitymanager. It would also contend for the single-writer lock against the user's own open project. Verified the sqlite rela-docs linked chromedp (3 deps).
severity: critical
resolution: 'Widened the refusal to `postgres || sqlite` and renamed the file capturer_database.go, with the reasoning spelled out separately for each build (postgres: shared DSN; sqlite: upward discovery plus lock contention). capturer_fs.go retagged to `!postgres && !sqlite` so it no longer claims fsstore for a build it does not cover. Verified the sqlite rela-docs now links no chromedp at all, and added the CI assertion that was already present for postgres.'
status: addressed
---
