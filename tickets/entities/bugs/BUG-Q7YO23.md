---
id: BUG-Q7YO23
type: bug
title: not has_relation() passes when outgoing counts are missing or partial
description: Incomplete relation counts (historical subject in affordances; a store error in transitionGraph.OutgoingCounts) read as zero, so not has_relation() opens a grant or transition.
priority: medium
status: backlog
---

## Description

Found in the TKT-205V2N security design review. Two paths answer an incomplete
relation count as "zero", so `not has_relation(...)` evaluates true and a grant
or transition opens:

- `affordances` `bindingContext.outgoingCounts` returns empty counts for a historical (versioned) subject.
- `appbuild.transitionGraph.OutgoingCounts` returns partial counts when the store returns an error mid-iteration.

Both should fail closed: an error, which denies the grant or blocks the
transition.
