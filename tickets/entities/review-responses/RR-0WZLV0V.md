---
id: RR-0WZLV0V
type: review-response
title: SSE stream advanced its day when re-deriving the gate failed
finding: On a failed re-derive the stream kept the prior gate and still advanced gateDay; so an expired grant kept sending change notices.
severity: minor
resolution: After a failed re-derive across a day change the stream sends nothing and retries on the next flush. TestSSE_FailedNewDayRegateWithholds.
status: addressed
---
