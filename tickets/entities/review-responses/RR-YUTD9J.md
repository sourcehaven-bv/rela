---
id: RR-YUTD9J
type: review-response
title: Windows reserved names
finding: The type-name pattern accepts con, nul and aux.
severity: nit
reason: cmdexec refuses to run on Windows unless the sandbox is turned off, so these names never reach a file write in a supported setup.
status: wont-fix
---
