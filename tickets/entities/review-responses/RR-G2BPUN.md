---
id: RR-G2BPUN
type: review-response
title: Control characters in source file names reach the terminal
finding: Skipped paths and errors print source-chosen file names with %s; an escape sequence could erase or forge report lines.
severity: minor
resolution: printImportReport replaces control characters in every string argument with U+FFFD. TestPrintImportReport covers an ESC sequence.
status: addressed
---
