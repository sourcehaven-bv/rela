---
id: RR-I32GPR
type: review-response
title: Schema and registration sites missing from the file list
finding: '(1) sqlitedb: the new table must also go into schemaSQL (fresh databases), not only into the v28->v29 rung, as channelReadsDDL does. The index can go there too, because no rung adds its columns. (2) pgstore/status_test.go pins the migration count (35) and must become 36. (3) sqlchannels `doc` (sql.go:160) is a separate struct from channels.Message and needs a Mentions field, or Message.Mentions never reaches the database. (4) docs-project GUIDE-realtime documents channel_reads and should get the mentions section too.'
severity: minor
resolution: Added schemaSQL, pgstore status_test (36), sqlchannels doc.Mentions and docs-project GUIDE-realtime to the files list.
status: addressed
---
