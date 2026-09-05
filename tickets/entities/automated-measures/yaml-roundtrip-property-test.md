---
id: yaml-roundtrip-property-test
type: automated-measure
title: 'Round-trip assertions at every serialization seam that accepts arbitrary user values'
kind: test
location: internal/markdown, internal/store/* (serialization seams)
status: proposed
description: >-
  BUG-B1RA3J had two failure shapes and only one of them errored. `"\n0"`
  produced a loud marshal failure; `"\n"` produced no error at all -- it emitted
  a block scalar that read back as the empty string, losing the value silently.
  A test asserting "write does not error" would have called the second a pass.

  The measure: any seam that serializes arbitrary user-supplied values must be
  covered by a test that marshals, unmarshals, and COMPARES -- not one that
  checks for an absent error. The comparison is what distinguishes "stored
  correctly" from "stored something".

  This generalizes past YAML. The sibling BUG-X7ICNM is the same class in JSON:
  encoding/json silently substitutes U+FFFD for invalid UTF-8 and reports
  success, so a write-only assertion passes over corrupted data there too.
---

## Why "no error on write" is the wrong assertion

It tests the code path, not the property. The property a store owes its caller
is that a read returns what a write was given; an error is only one of the ways
that can fail, and it is the *benign* way, because it is visible.

BUG-B1RA3J is a clean illustration: the failing write was the SAFE outcome
throughout. The dangerous version of that bug — a fix that silenced the error
while still emitting unreadable YAML — would have converted a loud failure into
a corrupt file, and every write-only test would have stayed green.

## Where this applies

Any seam converting user-controlled values into a serialization format:

- `internal/markdown.ValueToNode` (YAML frontmatter) — covered as of BUG-B1RA3J.
- `pgstore.marshalProps` / `unmarshalProps` (JSON) — NOT covered; BUG-X7ICNM is
  the known gap.
- Future backends added through the store conformance kit.

The conformance kit is the natural home: it already runs one suite against every
backend, and a round-trip property belongs there rather than in per-backend
tests, since divergence between backends is itself the bug class (both
BUG-B1RA3J and BUG-X7ICNM are backend divergences).

## Deliberately not proposed

A blanket "fuzz every seam" rule. The weekly sweep already exists and found
both of these. The gap was not fuzzing coverage — it was that the assertions
inside the fuzz target checked for errors rather than comparing values, so the
fuzzer could reach the silent case without reporting it.
