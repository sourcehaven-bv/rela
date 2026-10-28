---
id: RR-0RKY5W
type: review-response
title: Mention rows and the shared marker are keyed by raw principal.User, so an unmatched login equal to a person entity id receives that person's entries
finding: 'ListAddressed takes its target from callReader (internal/realtime/inbox.go:683, ReaderOf(authorOf(c.p))). With person mapping on, an unmatched login keeps its raw value as principal.User (internal/access/opener.go resolveIdentity falls back to the raw principal). If the IdP lets a user pick a username that passes entity.ValidateID and equals a person entity id (for example a self-registered username USR-7), that user''s inbox lists every unread mention of the real USR-7 on channels the attacker may read. Because markers are keyed by (user, org), the attacker''s mark_read also moves the real USR-7''s marker in the same org. That clears USR-7''s unread mentions and makes them evictable as read rows under Evict (internal/channels/addressed.go:230). The overlap between the raw-login and entity-id namespaces is older than this diff (slice 2 markers and post authorship share it); this diff adds mention delivery and eviction to what it reaches. Unverified: it depends on the IdP allowing a chosen username. Fix: separate the namespaces where person mapping is on (refuse or prefix an unmatched raw login that passes entity.ValidateID before it becomes principal.User), or track it as a follow-up with the forget-reader ticket TKT-6V8K8I.'
severity: minor
reason: The login-versus-entity-id key overlap predates this diff (internal/access/opener.go keeps an unmatched login as its key) and needs an IdP that allows usernames shaped like entity ids. Tracked in TKT-9KNO8X.
status: wont-fix
---
