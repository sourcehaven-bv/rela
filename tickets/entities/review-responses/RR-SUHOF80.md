---
id: RR-SUHOF80
type: review-response
title: Config field cap raised from 29 to 30
finding: CLAUDE.md prefers splitting a type over raising its plimsoll pin.
severity: nit
reason: Config mirrors the top-level keys of data-entry.yaml, so a new top-level key needs a field. Moving navigation, spaces and builtins into a sidebar struct would change the file format and is out of scope here.
status: wont-fix
---
