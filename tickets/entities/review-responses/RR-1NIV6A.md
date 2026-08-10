---
id: RR-1NIV6A
type: review-response
title: Duplicated coercion (predicatefns.bind vs affordances) already disagrees on []string
finding: 'bind.go doc claims the binder ''agrees by construction'' with affordances/bindings.go, but predicatefns.coerceList has a `case []string:` arm affordances lacks — a []string-stored prop binds a proper list in predicatefns vs a one-element [Nil] list in affordances. Not live (YAML decodes list props to []any where both agree), but the ''identical by construction'' invariant is already false, so drift is real. FIX (follow-up, not blocker): affordances delegates scalar/list coercion to predicatefns.EntityRecord (keeping its own current_user/has_* binding on top).'
severity: minor
status: open
---
