---
id: BUG-WQ7Y
type: bug
title: 'Architecture boundary violation: workspace depends on script'
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
`workspace` is allowed to depend on). Updated all call sites to explicitly
pass the script executor.
