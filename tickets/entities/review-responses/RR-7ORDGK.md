---
id: RR-7ORDGK
type: review-response
title: Validate RELA_RUN_AS only for commands that use an identity
finding: If localPrincipal validates in runKong unconditionally, a bad env value breaks rela version, completion and init. Validate after kong picks the command, or only on requiresProject paths and mcp.
severity: nit
resolution: RELA_RUN_AS is validated after command selection, only for commands that use an identity.
status: addressed
---
