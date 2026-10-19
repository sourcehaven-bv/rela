---
id: RR-YM39IT
type: review-response
title: Undecryptable fields can flip a match and post a spurious `leave`
finding: 'A git-crypt field the server cannot decrypt arrives as absent (`EntityHeader.Inaccessible`). It evaluates as "no match" without an Eval error, so §3 posts `leave` and later `enter` again. Treat a row whose rule fields are Inaccessible like an Eval error: do not change the entry.'
severity: minor
resolution: A row whose rule fields are Inaccessible leaves the entry unchanged.
status: addressed
---
