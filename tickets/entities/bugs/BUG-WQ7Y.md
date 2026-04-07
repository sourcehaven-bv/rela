---
id: BUG-WQ7Y
type: bug
title: 'Architecture boundary violation: workspace depends on script'
description: The workspace package imports script package, violating go-arch-lint rules
priority: medium
effort: s
why1: ScriptExecutor interface used script.Context as parameter type
why2: Context interface was defined in script package for convenience
why3: No separate interface package existed for shared types
why4: Shared abstractions were added in whichever package felt convenient instead of in a neutral/lower layer
why5: Architecture boundaries were not consulted when defining cross-package interfaces
prevention: Moved interface to metamodel package to follow architecture boundaries
status: done
---

The `workspace` package imports `script` package, violating the architecture
boundary defined in `.go-arch-lint.yml`. The `workspace` component is not
allowed to depend on `script`.

## Root Cause

The `workspace` package defined a `ScriptExecutor` interface that used
`script.Context` as a parameter type, creating an import dependency.

## Fix

Moved `ScriptContext` interface from `script` to `metamodel` package (which
`workspace` is allowed to depend on). Updated all call sites to explicitly pass
the script executor.
