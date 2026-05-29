---
id: RR-SQJU
type: review-response
title: Global-role checks need an entity-independent has_global_role func
finding: 'Follow-up to DR-C6 (crit round 2, Jeroen): once has_role is entity-scoped (3-arg), how does a predicate express a purely global-role check where no entity applies — e.g. gating a ''create'' button? Passing entity=nil into has_role is the nil-entity footgun the cranky review warned against.'
severity: significant
resolution: 'Added distinct has_global_role(current_user, role_name) bool rather than overloading has_role with a nullable entity. has_role stays the superset (global ∪ local); has_global_role is the global-only convenience form. Scope note: in THIS ticket every predicate is per-entity (field/relation affordances attach only to per-entity GET; create is gated by the phase-1 _actions/acl.AuthorizeWrite path, not the field resolver). Collection-scope predicates gating create are a future ticket; they''ll declare an env WITHOUT entity so only has_global_role is available — no nil entity ever constructed.'
status: addressed
---
