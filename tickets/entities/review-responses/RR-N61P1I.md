---
id: RR-N61P1I
type: review-response
title: Broken channel store turns a 403 into a 500
finding: openScope reads the channel store before the authorization pre-pass, so a store error gives 500 to a caller who would get 403.
severity: nit
status: open
---
