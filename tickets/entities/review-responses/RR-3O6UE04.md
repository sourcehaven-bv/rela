---
id: RR-3O6UE04
type: review-response
title: The CLI start error leaves no way out without a server
finding: 'On an fs project without a server, the message that the run will be retried while rela-server or the desktop app runs gives no action, and a per_subject: one run then blocks new starts.'
severity: minor
resolution: 'The message now also says: or cancel it with rela flow cancel <id>. flow_runs_test asserts it.'
status: addressed
---
