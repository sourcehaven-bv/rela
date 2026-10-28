---
id: RR-3EXS9B
type: review-response
title: 'The final-only in: check overstates "never postable"'
finding: 'on_final: close closes the channel through the outputs run after the move, not in the append step. Until that run completes, or after it fails and stays pending, the channel is open in the final state and such a type could be posted. The load refusal is still reasonable, but reword the reason to "the channel closes when the session enters these states, so the type would be offered at most until the close runs".'
severity: nit
resolution: The final-only reason now reads "the channel closes when the session enters these states, so the type would be offered at most until the close runs".
status: addressed
---
