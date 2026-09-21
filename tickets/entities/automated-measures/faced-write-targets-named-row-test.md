---
id: faced-write-targets-named-row-test
type: automated-measure
title: 'Test: a face-dropping write addresses the row the caller named, not the default'
description: A write that accepts or resolves a face must act on the row the caller named; where a method cannot express the face it must fail rather than silently retarget the default. Asserts the stored rows on both the named and default face, never the return value alone, because a wrong-row write returns success.
kind: test
location: internal/entitymanager/
status: proposed
---

## Measure

A write path that accepts or resolves a face must act on the row the caller
named. Where a method cannot express the face, it must **fail** rather than
silently retarget the default face.

## Why

`Manager.DeleteRelation` (`internal/entitymanager/manager.go:2022-2024`) passes
the zero face unconditionally, and the sibling doc comment (`:2029-2032`)
already records the consequence: it "deletes the default face's, and reports
success."

A create that cannot name a face fails loudly (`ErrFaceRequired`). A delete that
cannot name one succeeds against a different row and returns `true`. The second
is the dangerous shape, and it is invisible to a test that asserts only the
return value.

This generalizes `refusal-remediation-is-reachable-test` (BUG-64MU2Q) from "the
escape a refusal names must work" to "a write that cannot express its coordinate
must not quietly pick one".

## What the test asserts

For each write verb reachable from a client (create, update, delete, on both
entities and relations):

1. With rows or edges present on **both** a named face and the default face,
issue the operation addressed at the named face.
2. Assert the **named** row changed and the **default** row did not.

Assert the stored rows, never the return value alone — a wrong-row write returns
success, so an error-free assertion passes against the defect.

3. Where the API has no way to express the face, assert the call **errors**
rather than succeeding against the default.

## Applies to

`BUG-YVU8CP` (the `delete_relation` case), and as a regression floor for
`TKT-E9BAAA`'s create paths.
