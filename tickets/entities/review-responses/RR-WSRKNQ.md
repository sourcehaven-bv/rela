---
id: RR-WSRKNQ
type: review-response
title: Action names not discoverable in text output
finding: Docs and help said read --schemas lists actions; it lists only properties. MessageView.Actions appeared only in -o json, so a text user could not find action names.
severity: significant
resolution: 'read now prints a may: line per message (actions and edit/delete/remove/release, with the release time when set); docs and help point there; TestChannelCLI_Action asserts it.'
status: addressed
---
