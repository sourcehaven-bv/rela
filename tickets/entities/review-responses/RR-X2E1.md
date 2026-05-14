---
id: RR-X2E1
type: review-response
title: 'Code quality: server uses ent.IsLocked() but Mention semantics could be expressed without IsLocked entirely'
finding: 'Stylistic: collectMentions has the slightly unusual shape of calling `ent.IsLocked()` and then re-walking `ent.Inaccessible` inside `primaryInaccessibleReason`. Two passes over the same data. Either: (a) walk ent.Inaccessible once, returning both ''is any field locked'' AND ''first reason'', or (b) inline primaryInaccessibleReason as `if len(ent.Inaccessible) > 0 && ent.Inaccessible[0].Reason != ""`. The current shape isn''t wrong, but it imposes a tiny invariant (IsLocked => Inaccessible non-empty) that''s only documented in entity.go. The reviewer next year will wonder why we don''t just use the slice directly.'
severity: nit
resolution: lockedReasonFor does a single walk over e.Inaccessible returning (reason, locked) in one pass. buildMention no longer calls IsLocked() separately.
status: addressed
---
