---
id: RR-I3VIES
type: review-response
title: Channel writes under --run-as leave no asserted_by anywhere, which contradicts the docs
finding: 'asserted_by reaches only graph audit records. A channel post or edit writes no graph audit record, and the channel audit (filechannels create and moderator removal, ops.go:65/227) records By=author without the asserting OS user. A moderator removal done with --run-as therefore has no record of the person who performed it. docs/realtime.md says the OS user ''is recorded only in the graph audit log, as asserted_by'', which suggests every run-as action is traceable. Fix: carry asserted_by into the channel audit records for create and removal, or correct docs/realtime.md and docs/acl-security.md to say channel posts, edits and removals keep no record of the asserting user.'
severity: minor
resolution: 'Chose the docs route: realtime.md, acl-security.md and cli-reference.md now say that channel posts, edits, removals and creation keep no record of the OS user and that asserted_by is in graph audit records only.'
reason: 'Chose the docs route: realtime.md, acl-security.md and cli-reference.md now say that channel posts, edits, removals and creation keep no record of the OS user and that asserted_by is in graph audit records only.'
status: addressed
---
