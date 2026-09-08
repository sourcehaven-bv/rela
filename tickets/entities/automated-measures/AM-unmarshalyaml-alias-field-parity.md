---
id: AM-unmarshalyaml-alias-field-parity
type: automated-measure
title: Custom UnmarshalYAML alias structs stay in sync with their outer type
description: A reflection test asserting each type with a custom UnmarshalYAML has an inner alias covering every field of the outer struct. Guards the class where adding a field parses clean and arrives as the zero value, so a declared setting silently becomes its default.
kind: test
location: internal/metamodel/
status: proposed
---

## What it guards

The alias-struct pattern (needed to avoid recursing into `UnmarshalYAML`)
duplicates the field list, and YAML ignores unknown keys — so a field added to
the outer type but not the alias is dropped **silently**. The schema loads, the
value reads as unset, and the feature behaves as if the operator never wrote
the line.

Hit for real by `after:` on `CopyDef` (BUG-YAMLDROP): `after: discard` parsed
clean as `keep`, which would have turned a consuming promote into a
non-consuming one with no error anywhere.

## Shape

Table-driven over the affected types, comparing `reflect.NumField()` of the
outer type against its alias, failing with a message naming the fix. Follows
`internal/acl/ceilingguard_test.go`, which converts the same kind of
convention-only invariant into a failing test with an exemption list.
