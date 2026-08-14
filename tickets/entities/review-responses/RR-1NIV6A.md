---
id: RR-1NIV6A
type: review-response
title: Duplicated coercion (predicatefns.bind vs affordances) already disagrees on []string
finding: 'bind.go doc claims the binder ''agrees by construction'' with affordances/bindings.go, but predicatefns.coerceList has a `case []string:` arm affordances lacks — a []string-stored prop binds a proper list in predicatefns vs a one-element [Nil] list in affordances. Not live (YAML decodes list props to []any where both agree), but the ''identical by construction'' invariant is already false, so drift is real. FIX (follow-up, not blocker): affordances delegates scalar/list coercion to predicatefns.EntityRecord (keeping its own current_user/has_* binding on top).'
severity: minor
reason: 'Doc corrected: predicatefns/bind.go no longer claims ''identical by construction'' — it now states the two binders agree only for markdown-frontmatter value shapes (list props decode to []any), notes affordances'' coerceList lacks the []string fast-path, and flags that verdict-affecting changes here must be mirrored in affordances until they''re unified. The full merge (affordances delegates scalar/list coercion to predicatefns.EntityRecord, keeping its own current_user/has_* bindings) is a tracked follow-up — not blocking phase 2b since the divergence is not reachable from YAML-decoded entities. Belongs with a broader affordances-binder refactor.'
status: deferred
---
