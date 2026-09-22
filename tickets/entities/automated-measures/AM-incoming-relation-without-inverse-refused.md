---
id: AM-incoming-relation-without-inverse-refused
type: automated-measure
title: An incoming form relation with no declared inverse is refused rather than thrown on
description: A config-load test asserting that a form declaring an incoming relation whose type has no declared inverse is rejected with a clear error, rather than loading and throwing at submit inside buildRelationsPatch. Not yet written — BUG-2XN24C is backlog.
kind: test
location: internal/dataentryconfig/validate_test.go
status: proposed
---

A config-load test asserting that a form declaring an `incoming` relation whose
relation type has no declared `inverse:` is refused at load with an error naming
the relation.

Today such a form loads, and the failure surfaces only at submit, as an
unhandled throw from `buildRelationsPatch` ("no inverse declared in metamodel …
DynamicForm should have pre-flighted this") — a pre-flight that does not exist.

Writing this test is the verification for BUG-2XN24C. If the fix instead lands
on the SPA side (a load-time refusal in DynamicForm), the measure moves to the
frontend suite; the property being pinned is the same either way.
