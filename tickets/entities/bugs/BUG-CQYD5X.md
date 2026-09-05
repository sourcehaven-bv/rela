---
id: BUG-CQYD5X
type: bug
title: 'PropertyValues: only sqlitestore enforces storeutil.ValidateProperty on the property name'
description: 'storeutil.ValidateProperty rejects empty names and names containing a slash (attachment-key collisions). sqlitestore applies it in PropertyValues and returns "store: empty property name" while fsstore / memstore / pgstore accept the same call and return an empty result. Entity writes with such a key succeed on all four backends. Found by the FuzzPropertyValuesTypeZoo round-trip work for BUG-X7ICNM which skips such names until the backends agree on whether the rule is a write-time gate or a read-time gate or neither.'
priority: low
status: backlog
---

Also seen: PropertyValues with a property name that is not valid UTF-8 errors on
pgstore (Postgres rejects the query parameter) while the other backends return
an empty result. The write gate (storeutil.ValidateProperties) now refuses such
a name on entity writes so it cannot be stored; the read-side divergence remains
and belongs to the same decision as the empty-name case.
