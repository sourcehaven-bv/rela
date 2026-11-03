---
id: RR-4Y8AW46
type: review-response
title: fs rewrite error fails a rename that already took effect
finding: rewriteMentions runs after the files moved and the events fired. A read or write error on one mentioning body makes RenameFamily return an error for a rename that happened; a retry gets ErrNotFound.
severity: significant
resolution: 'The fs rewrite runs after the commit point as best effort: a failed body logs a warning and the rename succeeds.'
status: addressed
---
