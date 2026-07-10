---
id: AM-acl-neighbor-read-leak-invariant
type: automated-measure
title: 'Integration test: no neighbor-emitting read endpoint leaks a hidden neighbor'
description: Enumerates every neighbor-emitting read endpoint (list, ?include=, /relations, /relations/{type}, single-GET) with a hidden-neighbor fixture and asserts no hidden peer's id/type/meta appears on any of them. Catches any future neighbor-emitting endpoint that fails open (the class that produced BUG-ABXMAV). P4 invariant test.
kind: test
location: internal/dataentry (integration test, added in the BUG-ABXMAV fix PR)
status: proposed
---
