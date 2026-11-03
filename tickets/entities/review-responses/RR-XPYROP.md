---
id: RR-XPYROP
type: review-response
title: Compiled accessors deep-copy the session on every call
finding: 'ChannelType() and ChannelTypes() clone every type for every head, post and list call. A session of up to 64 states with on: maps adds an allocation-heavy copy to the list path, although it adds no store calls. Make Session immutable by construction (unexported fields, read-only accessors, as predicate programs already are) and share the pointer in clone. AC 5 then checks that a caller cannot change it, not that it is copied.'
severity: nit
resolution: 'Plan updated: the compiled Session is immutable with read-only accessors and the pointer is shared on clone (AC 7).'
status: addressed
---
