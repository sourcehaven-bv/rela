---
id: refusal-remediation-is-reachable-test
type: automated-measure
title: 'Test: a refusal''s recommended alternative address must actually work'
description: 'Secondary measure for BUG-64MU2Q. The faced-PATCH test asserted only that the 422 face_relations_unsupported fires, never that the escape its detail recommends can be followed — so when BUG-HC6I2T removed the bare address the advice silently became impossible and every test still passed. Where a refusal names an alternative, the test must perform that alternative and assert it succeeds. Once BUG-64MU2Q''s fix lands the guard disappears and this becomes the positive case: a faced relation write succeeds and the edge lands on the addressed face, not the zero face.'
kind: test
location: internal/dataentry/entityref_test.go
status: proposed
---

## Why

An error's remediation text is prose in a `fmt.Sprintf`, so nothing ties it to
the model it describes. Removing the address it names — as BUG-HC6I2T did with
`bare_face` — breaks no compile and fails no test, because the guard's test
asserted only that the refusal fired.

## What it pins

Whenever a refusal tells the caller to do something else, the test performs that
something else and asserts it succeeds. A refusal that names an alternative is a
contract about that alternative.

## After the fix

BUG-64MU2Q removes the guard by making the faced relation write work. The test
then asserts the positive: the write succeeds and the edge is stored on the face
that was addressed, never the zero face.
