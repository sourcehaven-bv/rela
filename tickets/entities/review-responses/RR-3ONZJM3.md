---
id: RR-3ONZJM3
type: review-response
title: Present mode has no Exit when the page anchor is missing
finding: With ?present=1 on an entity page whose entity is missing or hidden, the shell hid the sidebar but PageSteps was not drawn, so there was no control to leave present mode.
severity: significant
resolution: presenting now also requires a present anchor; test 'does not present an entity page whose entity is missing'.
status: addressed
---
