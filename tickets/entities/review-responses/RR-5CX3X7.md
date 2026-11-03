---
id: RR-5CX3X7
type: review-response
title: Integration comment does not record who asserted it
finding: Comments have no audit log, so the OS user behind --run-as was not recorded (also raised by the security review).
severity: minor
resolution: Stdio AddComment logs entity, comment id, author and asserted_by; ValidAuthor godoc and docs/comments.md say the name is a self-asserted label.
status: addressed
---
