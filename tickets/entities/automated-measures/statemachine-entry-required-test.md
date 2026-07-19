---
id: statemachine-entry-required-test
type: automated-measure
title: 'Test: state machine with transitions requires an entry value (no unconstrained create)'
description: Asserts Compile rejects a CustomType with transitions but no initial/default, so a create can never enter a machine unconstrained (BUG-X1C7S / rela#1146). Complemented by TestTransition_IllegalEntryOnCreateIs422 (non-initial create value -> 422).
kind: test
location: internal/statemachine/statemachine_test.go:TestCompile_RejectsTransitionsWithoutEntry
status: active
---
