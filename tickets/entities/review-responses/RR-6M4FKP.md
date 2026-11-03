---
id: RR-6M4FKP
type: review-response
title: Temp working dir changes existing cmdexec callers
finding: Setting Cmd.Dir for every Run breaks relative program paths and relative resource paths of existing transforms, scans and document commands on macOS.
severity: significant
resolution: The temp working dir now applies only to RunWith; Run keeps the inherited directory, so existing callers are unchanged.
status: addressed
---
