---
id: RR-MVS2MH
type: review-response
title: Comment watch poll cost on the remote server
finding: Each list_comments target replays on every channel poll (2s). Worst case 32 watches x 16 targets per server. The plan should state this bound.
severity: minor
resolution: 'Risks section states the bound: 16 targets per watch and 32 watches per server.'
status: addressed
---

Each list_comments target replays on every channel poll (2s). Worst case 32
watches x 16 targets per server. The plan should state this bound.
