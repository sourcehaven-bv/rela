---
id: RR-U9IYRL
type: review-response
title: Data copy follows symlinks in the source
finding: fsstore, filecomments and filemigstate read through OsFS, which follows a symlink. A cloned repo with attachments/DOC-1/file/key.pem pointing at ~/.ssh/id_ed25519 puts the key into rela.db. Planned in RR-JUK3CT but not implemented.
severity: significant
resolution: checkSymlinks (fsimport/symlinks.go) refuses any symlink under entities, relations, attachments, .rela/comments, .rela/migration and migrations/applied.json before the source is opened. TestRun_RefusesSymlinksInData.
status: addressed
---
