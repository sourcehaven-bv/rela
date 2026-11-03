---
id: RR-YB41PM
type: review-response
title: Service.Add refuses every job identity as author
finding: 'comments.Service.Add (authorFrom) refuses any principal.IsReserved user, which covers every system: and integration: name. Every --run-as identity is one of those, so the planned stdio add_comment could never store a comment.'
severity: significant
resolution: 'Plan step 0: comments.ValidAuthor admits integration: job identities and keeps refusing system: ones.'
status: addressed
---

comments.Service.Add (authorFrom) refuses any principal.IsReserved user, which
covers every system: and integration: name. Every --run-as identity is one of
those, so the planned stdio add_comment could never store a comment.
