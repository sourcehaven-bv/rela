---
id: RR-OX8G0N
type: review-response
title: Shared-manager gate breaks CalDAV with a 409 loop
finding: CalDAV PUTs carry unchanged read-only fields and server defaults; the manager gate refused them and CalDAV mapped the error to 409.
severity: critical
resolution: 'Redesign: the gate applies only on the entitymanager.FieldGated handle, given to rela-server MCP and scheduled Lua. dataentry (CalDAV, clone, provisioner, webhooks, actions) keeps the default handle and its own validateFieldWrite.'
status: addressed
---
