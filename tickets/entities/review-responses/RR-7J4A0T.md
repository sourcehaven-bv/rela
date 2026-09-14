---
id: RR-7J4A0T
type: review-response
title: 'Status built a file: URI DSN by concatenation and read the wrong database'
finding: 'Status used `"file:"+path+"?mode=ro"`. The file: scheme is required to pass mode=ro, and it is also what puts SQLite into URI mode, where ''#'' starts a fragment and ''%'' starts a percent-escape. A project at ~/proj#2/.rela/rela.db therefore addressed a DIFFERENT file: no error, version reported as 0, and under this driver mode=ro CREATED the truncated path as a new file. `rela db status` would print ''Database is BEHIND: schema version 0'' for a current database, exit 1 and break the CI gate; `rela db migrate` would then read before=0, connect (migrating nothing), and print ''Applied migrations: schema version 0 to 2'' about a file it never touched.'
severity: critical
resolution: 'Extracted readOnlyDSN, which builds the URI with net/url (Opaque set to an EscapedPath, RawQuery mode=ro) instead of concatenating. Reproduced the original bug in a standalone program first — a#b.db read back version 0 and left a stray file named ''a'' in the directory. TestStatusHandlesURIMetacharactersInPath covers #, %2F, space and ?; it fails on the reverted implementation for the # and %2F cases.'
status: addressed
---
