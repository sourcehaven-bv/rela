---
id: RR-V3UT
type: review-response
title: luaMdList docstring not updated for table item form
finding: The doc comment on luaMdList only describes string items but it now also accepts task tables. Public API addition with no documentation update.
severity: significant
resolution: luaMdList docstring rewritten with the full task item table shape, all three usage forms (plain, ordered, task), and explicit field requirements.
status: addressed
---
