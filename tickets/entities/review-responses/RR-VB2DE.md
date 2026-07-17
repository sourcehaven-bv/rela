---
id: RR-VB2DE
type: review-response
title: Compile does not validate Default-as-entry-value against declared values (fail-fast gap)
finding: 'compile.go:78-86: entry = Initial else Default, but the membership check validates only ct.Initial. A machine with initial unset and a typo''d default (e.g. default: aproved) compiles with no boot error, violating the ''malformed machine fails fast at boot'' promise. The bad entry is caught only as a soft downstream enum warning. Fix: validate the resolved entry value (if entry != '''' && !values[entry]) or specifically validate ct.Default when it is the entry source.'
severity: significant
resolution: compile.go now validates the RESOLVED entry value (Initial-or-Default) against declared values, naming the source (initial/default) in the error. A typo'd default-as-entry now fails fast at boot. Test TestCompile_RejectsUndeclaredDefaultEntry.
status: addressed
---

## Finding

`compile.go:78-86`: `entry = Initial` else `Default`, but the membership check
at line 83 validates **only** `ct.Initial`. When `Initial == ""`, `entry`
becomes `ct.Default` unchecked.

A machine with `initial:` unset and `default: aproved` (typo) compiles with no
boot error — violating `Compile`'s "malformed machine fails fast at boot, never
at write time" contract. The bad entry surfaces only as a soft downstream enum
warning, and creates still pass (default applied, then `got == m.entry` matches
the same typo).

## Resolution

Validate the resolved entry value: `if entry != "" && !values[entry]` → problem,
or specifically validate `ct.Default` when it is the entry source.
