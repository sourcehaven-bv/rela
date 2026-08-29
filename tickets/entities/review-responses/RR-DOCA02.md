---
id: RR-DOCA02
type: review-response
title: a typo'd type= makes the negative shows{} claims vacuous
finding: type= was passed straight into store.EntityQuery; an unknown type yields an empty set, which satisfies both exactly={} and any absent= claim. Those are precisely the claims the docs recommend. contains= fails safe, which is why the hole was easy to miss.
severity: critical
resolution: type= is validated against the metamodel in both shows{} and the authorization verbs, listing declared types on failure.
status: addressed
---
