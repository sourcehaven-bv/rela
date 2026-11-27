---
id: BUG-9DMQUIB
type: bug
title: 'A roll-up where: and related() ignore the property default'
description: 'where: {blocked: false} only matched entities where false is stored. Entities that leave blocked unset (default false) were missed.'
priority: high
effort: m
why1: wherePredicate and relresolve.Hop compared only the stored value, so an unset property never equalled its default.
why2: The store had no operator for equal-or-unset, so the default could not be expressed in a pushed-down query.
why3: Defaults are applied on write for some types but not stored for others, so stored data and declared defaults diverge.
why4: No test compared a constraint on the default against an entity that leaves the property unset.
why5: Default semantics were defined per surface rather than once in the metamodel.
prevention: metamodel.IsDeclaredDefault is the one definition; storetest and the ACL tests pin unset, empty and default values on every backend.
started: "2026-11-27"
completed: "2026-11-27"
status: done
---
