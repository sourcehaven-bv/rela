---
id: RR-LBXIB2
type: review-response
title: 'Delivered-set gap-closure is unsound: relation/ancestor/policy transitions fire no entity event, leaving stale true membership → delete leaks'
finding: 'The delivered-set''s invariant ''membership IS the verdict'' is false. It depends on every readable→unreadable transition firing an entity:updated event so the handler can evict. But readability is a function of role-relation edges, inherit_roles_through ancestry, group membership, and the policy file — NONE of which produce an entity:updated on the target. pumpStoreEvents (watcher.go:183) drops all relation events. Attack: entity created-readable (added to set) → admin deletes the conferring role-relation (EventRelationDeleted, dropped, no eviction) → admin deletes entity → delete delivered because id still in set → leaks {type,id} of a walled-off entity. Policy-reload variant: an acl.yaml reload narrowing a role fires no store event at all; every accumulated id stays true forever. The delivered-set is a cache of stale verdicts under an OLD graph/policy with no invalidation on the dominant transition paths.'
severity: critical
resolution: 'Delivered-set design ABANDONED. Correlation moves to the client (cacheId→entity map) where it''s safe (client isn''t an ACL authority), and the delete wire payload is an opaque per-principal HMAC cacheId. The server keeps no per-connection known-set, so there is no stale-verdict cache to go wrong on relation/ancestor/policy transitions. The leak this finding identified cannot occur: a delete carries no real id, only a cacheId reversible solely by a holder who legitimately received the create.'
status: addressed
---
