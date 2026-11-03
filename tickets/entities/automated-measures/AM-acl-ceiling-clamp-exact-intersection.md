---
id: AM-acl-ceiling-clamp-exact-intersection
type: automated-measure
title: 'Exhaustive test: ceiling clamp equals role grants ∩ ceiling'
description: Enumerates every subset of wildcard, type and face role entries against every allow and deny ceiling over a small type/face universe, and asserts the clamped role grants a (type, face) exactly when the role grants it and the ceiling permits the type.
kind: test
location: internal/acl/ceilingfacegrants_test.go (TestClamp_IsExactIntersection)
status: active
---
