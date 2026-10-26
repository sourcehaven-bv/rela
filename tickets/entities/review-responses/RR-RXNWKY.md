---
id: RR-RXNWKY
type: review-response
title: Mark Writer immutability, and the timer writer, are not specified
finding: 'CheckOutputStep allows claims and progress writes, but the plan does not say that Writer, the generation and the transition coordinates are immutable after the step that creates the mark. A future caller could otherwise rewrite who a retry acts as. For timer marks, the retry recomputes the outputs from the current config but takes the writer from the mark, so a reload that changes run_as leaves retries acting as an identity the config no longer names. Plan change: CheckOutputStep refuses any change to Writer and the identifying fields (add a row in testOutputsRefusals). For timer marks, use the current config''s run_as and keep the stored Writer for display only, or fail the mark with `run_as_changed`.'
severity: minor
resolution: CheckOutputStep refuses changes to Writer, Roles, Gen, From, To and Cause, with a testOutputsRefusals row. The timer-writer half is deferred to 5b.
status: addressed
---
