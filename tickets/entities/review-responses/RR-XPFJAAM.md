---
id: RR-XPFJAAM
type: review-response
title: File store GetMessages ignores ctx
finding: One call can parse up to 200 full segments; a timed-out live round still runs it to the end. Check ctx.Err() per channel.
severity: minor
resolution: GetMessages checks ctx.Err() before each channel; TestGetMessages_Canceled pins it.
status: addressed
---
