---
id: BUG-L2LQZX8
type: bug
title: Mapped import prints import-file text to the terminal unfiltered
description: Control characters from the import file reached the terminal and the JSON output, so they could hide plan lines.
priority: medium
status: backlog
---

## Summary

`rela import --mapping` printed text from the import file to the terminal
without filtering. The `-o json` output did not go through `terminalSafeJSON`. A
control character in a key or value could hide or overwrite lines of the dry-run
plan, which is the check before applying.

## Fix

The plan and result lines go through `printable()`, and the JSON output through
`terminalSafeJSON()`. `TestImportMapped_ControlCharacters` puts ESC, CR, a C1
CSI and a bidi override in the key and values.

GitHub: #2325
