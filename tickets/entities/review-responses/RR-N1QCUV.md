---
id: RR-N1QCUV
type: review-response
title: Token key and DSN inherited by child processes
finding: RELA_TOKEN_KEY and RELA_DATABASE_URL reach cmdexec converters and data-entry commands.
severity: minor
resolution: cmdexec.Environ strips RELA_TOKEN_KEY and RELA_DATABASE_URL; used by cmdexec, data-entry commands and graphviz; tests.
status: addressed
---
