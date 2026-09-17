---
id: AM-validation-message-description-separate
type: automated-measure
title: A Lua validation rule's per-entity message and the rule's description are asserted to coexist
kind: test
location: internal/validation/lua_test.go (TestLuaValidation_MessageAndDescriptionAreSeparate, TestNonLuaViolation_HasNoMessage)
status: active
description: >-
  Two tests pin the two fields apart on one violation. The first runs a Lua rule
  that returns a DIFFERENT message per entity and asserts each violation carries
  its own Message AND the shared rule Description — so folding either field back
  into the other fails it. The second runs a non-Lua rule and asserts Message is
  empty, which is the contract every renderer relies on to omit the message
  rather than print a dangling separator.
---

## Why this shape

The bug (BUG-6LCF9L) was invisible to the existing Lua tests because they
asserted only the script's message and used `Description` as the place to find
it. That passes whether or not the rule description survives, so the suite
encoded the defect as expected behaviour.

The guard is therefore "both fields, one violation" rather than "the message is
reported". Asserting only the message reintroduces the blind spot; asserting
both makes the two cardinalities — per-rule and per-violation — structurally
observable.

Two entities failing the SAME rule for different reasons is the case the first
test fixes in place, because that is precisely what the operator could not tell
apart. A single-violation test would pass against a renderer that showed one
arbitrary entity's message as the rule heading.

The empty case is a separate test rather than a branch: "no message" is a
contract renderers depend on, not an edge case of the first assertion.
