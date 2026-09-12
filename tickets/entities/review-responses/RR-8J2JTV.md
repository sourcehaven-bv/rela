---
id: RR-8J2JTV
type: review-response
title: Guard scoped to _test.go left tagged production files uncovered
finding: scripts/check-tagged-tests.sh discovered tags only from *_test.go, but `sqlite` and `memorybackend` gate PRODUCTION files (internal/appbuild/appbuild_sqlite.go, internal/cli/db_sqlite.go, internal/docscli/capturer_sqlite.go, internal/appbuild/appbuild_memory.go). Those are skipped by the default build for exactly the reason the script's own header describes, so they rot by the same mechanism. The guard was scoped to a strict subset of the code it was built to protect.
severity: significant
resolution: 'Widened discovery to all *.go files. The tag set went from 3 to 5: maildemo, mailmanual, memorybackend, postgres, sqlite. Both newly covered tags vet clean today, so this is prevention rather than a fix. Comments, messages and the justfile description updated from "test files" to "files".'
status: addressed
---
