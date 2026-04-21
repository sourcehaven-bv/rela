---
id: RR-GQ5S8
type: review-response
title: GetIDPrefixes silently drops id_prefixes when both id_prefix and id_prefixes are set
finding: 'EntityDef.GetIDPrefixes returns [IDPrefix] first and only falls back to IDPrefixes if IDPrefix is empty. Pre-existing behavior, not introduced here. But this PR is the first thing surfacing IDPrefixes as a first-class multi-value concept to API callers and the frontend, so it makes the trap more visible: a user adding multi-prefix support to an existing entity type might leave id_prefix set and silently lose the rest.'
severity: significant
reason: Out of scope for this ticket. The trap is metamodel-loader behavior that pre-dates this work and affects more than just data-entry; fixing it (loader-time error or union semantics in GetIDPrefixes) belongs in its own ticket so the metamodel + entitymanager + Lua bindings can all be updated and tested together. Filing as a follow-up; this PR only consumes GetIDPrefixes, doesn't modify it.
status: deferred
---
