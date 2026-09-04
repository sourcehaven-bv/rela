---
id: AM-world-default-face-name-resolves
type: automated-measure
title: A world selecting a default-marked face BY NAME resolves to that face
kind: test
location: internal/worlds/ (tests to be written with the fix)
status: proposed
description: "A world whose select: chain names a face declared with `default: true` must compile to a chain that matches that face, not to an unmatchable one. Pins BUG-DFLTCHAIN, where the two name-to-coordinate mappings disagreed about default-marked faces and the world silently resolved to nothing."
---

Pins BUG-DFLTCHAIN.

The two functions that map a declared face name to a stored coordinate must
agree about `default: true`. A world naming such a face must serve it, and a
test must fail if the compiled chain can never match.

Asserting on the RESOLVED FACE rather than on the compiled chain's shape: a
test pinning the internal representation would pass against a chain that is
well-formed and still unmatchable, which is exactly the defect.
