---
id: RR-KAFSNJ2
type: review-response
title: Previous state filtered in both app and library
finding: channelView.ts drops an undeclared previous; the component also looks it up in states.
severity: nit
reason: 'Each layer guards its own input: the app maps the wire, the library cannot trust its caller. The cost is one Map lookup.'
status: wont-fix
---
