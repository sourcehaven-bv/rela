---
id: RR-LW0XX6
type: review-response
title: Locked content only checked in a pre-scan
finding: sqlitestore does not reject Inaccessible stubs and canonical hashing ignores Inaccessible, so content locked after the pre-scan would be written empty and pass verify.
severity: significant
resolution: 'Plan updated: IsLocked is checked in the write loop and in verify too; attachments are checked for the git-crypt header.'
status: addressed
---
