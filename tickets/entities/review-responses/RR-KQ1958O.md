---
id: RR-KQ1958O
type: review-response
title: CLI download writes partial files and binary to a terminal
finding: channel file --to left a partial file on error, and with no --to wrote raw bytes to a terminal.
severity: nit
resolution: --to writes a temp file and renames it; without --to the command refuses a terminal stdout, and --to - forces stdout. Tested in TestWriteFileAtomic.
status: addressed
---
