---
id: RR-KG267AN
type: review-response
title: A disconnected client logged a warning
finding: A cancelled request made the title lookup fail and log a warning.
severity: nit
resolution: changeTitles returns early when ctx.Err() is set.
status: addressed
---
