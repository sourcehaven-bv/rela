---
id: AM-acl-audit-permission-consumers-complete
type: automated-measure
title: A7 sees every data-entry permission gate, and never asserts 'dead' when it cannot see them
description: 'Two-part test. (1) Per-surface coverage: for each data-entry gate that carries a permission (documents, dashboard cards, navigation, commands), a permission referenced ONLY by that surface emits no A7 finding — so dropping one surface from the CLI adapter fails a test rather than silently reporting live config as dead. (2) Fail-safe: with no PermissionConsumer injected, A7 must not emit a ''dead'' finding, so an audit path that cannot load the data-entry config degrades to skip/cannot-verify instead of reproducing the false positive.'
kind: test
location: internal/aclaudit + internal/cli (adapter coverage per gate surface; nil-consumer skip)
status: proposed
---

Guards the why5 of the UI-gates bug: aclaudit's "inject a narrow view of what
you need" posture was applied to the metamodel but never carried to permission
consumers, so A7 asserted a whole-system fact while seeing one file.

Part (1) is per-surface deliberately. A single test using one gate type would
pass while three of the four surfaces were missing from the adapter — which is
the same class of incompleteness that caused the original bug.

Part (2) is the load-bearing half. The existing `MetamodelReader` precedent is
that nil is valid and simply drops checks; copying that for permissions would
mean a nil consumer runs A7 blind and re-emits the false "dead" report. This
pins the divergence so a later refactor cannot quietly restore symmetry with the
metamodel path.
