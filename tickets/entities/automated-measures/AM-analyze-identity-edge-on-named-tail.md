---
id: AM-analyze-identity-edge-on-named-tail
type: automated-measure
title: 'Check: no identity-scoped edge sits on a named face'
description: An analyze check that reports an identity-scoped relation whose tail is a named face. Catches edges an earlier face move put on a face where they cannot be updated and are removed with the face (BUG-KOTY66).
kind: test
location: internal/analyze (planned)
status: proposed
---
