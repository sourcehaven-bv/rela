---
id: RR-13PAYD0
type: review-response
title: Ordinary-edit change events could move a run twice
finding: Once TKT-FWQZGAM sends ordinary edits as change events, the step write itself could be judged against the next state.
severity: significant
resolution: Change takes the generation it saw and refuses another. The step write carries the flow step on ctx, and TKT-FWQZGAM's hook ignores writes with a flow step, as it does for outputs (RR-EMIUGI8). Written into both tickets.
status: addressed
---
