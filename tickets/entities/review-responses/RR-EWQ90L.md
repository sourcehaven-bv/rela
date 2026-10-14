---
id: RR-EWQ90L
type: review-response
title: Nil policy fails per request rather than at startup
finding: attachACLRequest answers 500 on every request when NewOpener fails, instead of failing at wiring.
severity: minor
reason: 'Unreachable: responseGates installs the middleware only for a non-nil Declarative. Changing attachACLRequest to return an error ripples through router wiring; the access-service PR of TKT-WFS778 replaces this wiring.'
status: deferred
---
