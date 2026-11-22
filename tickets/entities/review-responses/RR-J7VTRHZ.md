---
id: RR-J7VTRHZ
type: review-response
title: "Commands stay in a read-only workspace"
finding: "Lua commands still show in a read-only workspace."
severity: nit
resolution: "Won't fix: a command may only read; the server refuses its writes with tenant_read_only."
reason: "A command may only read; the server refuses its writes."
status: wont-fix
---
