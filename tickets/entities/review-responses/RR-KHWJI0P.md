---
id: RR-KHWJI0P
type: review-response
title: The task list fails past its candidate cap
finding: Taking every readable input_required header as a candidate makes "my tasks" fail for everyone once there are more than the cap.
severity: critical
resolution: Candidates are narrowed in the store to the flows where one of the caller's roles (held globally, or conferrable through a role relation) is waited for, on top of the row gate and status. Past the cap the list is served truncated with a flag computed after the gate, not an error; a flow filter narrows further.
status: addressed
---
