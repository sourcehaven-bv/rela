---
id: RR-CODIYGN
type: review-response
title: ParseFlow validates the stored outside_edits of a run
finding: ParseFlow compiled outside_edits from a run's stored declaration, so a rule that today's schema rejects could strand the run, while the rules in force come from the current flows.yaml anyway.
severity: minor
resolution: The loader skips outside_edits on a stored load. TestParseFlow_IgnoresStoredOutsideEdits pins it.
status: addressed
---
