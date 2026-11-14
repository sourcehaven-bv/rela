---
id: RR-AS8DJEM
type: review-response
title: Delete edit check cannot rely on the diff hash
finding: The plan judged a remote edit of a locally deleted row by the reconcile diff. After a push the ledger hash is zero until the next pull, and relation rows keep no ETag, so a delete right after a push would always look like a remote edit, and relations could not be sent with If-Match.
severity: significant
resolution: Keep the base digests on a deleting row until the delete is sent. Before the DELETE the run fetches the remote row with its ETag and compares the mapped fields with the base, then sends DELETE with If-Match on the fetched ETag; a 412 fetches once more.
status: addressed
---
