---
id: RR-JD6UTYA
type: review-response
title: Read-only mode fails every capture run
finding: Under --read-only the append is denied, so each run fails into the retry ladder.
severity: nit
reason: Failing closed is correct under ReadOnlyACL; nothing is stored or disclosed, and the retry ladder bounds the log noise.
status: wont-fix
---
