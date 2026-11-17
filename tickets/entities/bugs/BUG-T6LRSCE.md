---
id: BUG-T6LRSCE
type: bug
title: Long digit string in an integer property fails Postgres integer queries
description: A string of more than 131072 digits in an integer property made the Postgres cast to numeric fail, so every integer query on that property returned an error (GitHub issue 2427).
priority: low
effort: xs
why1: The string branch of integerExpr cast any run of digits to numeric, and numeric cannot read more than 131072 digits.
why2: The regex that guards the cast checked the format of the string but not its length.
why3: The design treated a format check as enough to make a cast safe, and the differential tests only used short values.
why4: No test stored a value at the limits of the PostgreSQL type.
why5: Casts in generated SQL had no shared rule that a guard must bound the length as well as the format of the value.
prevention: TestIntegerExprAgreesWithGo now stores strings of 200000 digits and many leading zeros. A guard before a SQL cast bounds both format and length.
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

## Description

In `integerExpr` (pgstore), the string branch tested a string with
`^[+-]?[0-9]+$` and cast it to `numeric` without a length bound. A string of
more than 131072 digits made the cast fail, so every Postgres query with an
integer comparison on that property returned an error. GitHub issue #2427.
