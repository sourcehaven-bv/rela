---
id: TKT-4BQFV
type: ticket
title: Improve error messages for metamodel YAML type mismatches
kind: enhancement
priority: medium
status: backlog
---

<!-- github-issue:148 -->

Imported from [#148](https://github.com/sourcehaven-bv/rela/issues/148)

## Problem

When a metamodel has YAML type mismatches (e.g., using a string where an array
is expected), the error messages are not very helpful for users unfamiliar with
the internal Go types.

## Example Error

When loading a metamodel with relations defined like this (using strings instead
of arrays for `from` and `to`):

```yaml
relations:
  deploys:
    description: "Client deploys an application instance"
    from: client        # ❌ Should be: [client]
    to: application     # ❌ Should be: [application]
```

The error message is:

```
failed to load metamodel: yaml: unmarshal errors:
  line 93: cannot unmarshal !!str `client` into []string
  line 94: cannot unmarshal !!str `applica...` into []string
```

Similarly, when `validations` is defined as a map instead of an array:

```yaml
validations:
  storage-must-have-backup:    # ❌ Should be: - name: storage-must-have-backup
    name: storage-must-have-backup
    description: "..."
```

The error is:

```
failed to load metamodel: yaml: unmarshal errors:
  line 12: cannot unmarshal !!map into []metamodel.ValidationRule
```

## Suggested Improvement

Provide more user-friendly error messages that:

1. **Identify the field name** that has the wrong type
2. **Explain what was expected** in plain English
3. **Show what was found** in the YAML
4. **Suggest the correct format** with an example

For example:

```
Error loading metamodel at line 93:

  Field "from" in relation "deploys" expects an array of entity types, but found a string.

  Found:    from: client
  Expected: from: [client]

  See: https://github.com/sourcehaven-bv/rela/blob/develop/docs/metamodel.md#relations
```

Or for validations:

```
Error loading metamodel at line 12:

  The "validations" field expects an array of validation rules, but found a map.

  Found:
    validations:
      storage-must-have-backup:
        name: ...

  Expected:
    validations:
      - name: storage-must-have-backup
        ...

  See: https://github.com/sourcehaven-bv/rela/blob/develop/docs/metamodel.md#validation-rules
```

## Context

These errors commonly occur when:
- Users are new to Rela and learning the metamodel format
- Copying examples that may use shorthand notation
- Migrating from other tools with different YAML conventions

Clearer error messages would significantly improve the onboarding experience.
