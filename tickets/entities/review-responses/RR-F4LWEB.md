---
id: RR-F4LWEB
type: review-response
title: Ref refusal reason talks about text the clicker never sees
finding: 'internal/realtimecfg/buttons.go:144 gives the markdown reason for both ref and markdown properties. The docs (GUIDE:~897) give a different reason for ref (the entity cannot be checked). Fix: use one reason per type, matching the docs.'
severity: nit
resolution: 'The refusal now gives one reason per type: ref says a click cannot check the entity it names, markdown says a click posts text the clicker never sees, matching the guide. Tests assert both reasons.'
status: addressed
---
