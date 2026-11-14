---
id: RR-KCYRSF7
type: review-response
title: Tool names checked against one build's registrations
finding: toolFilter.check judged names against what this server registered, so a deny entry for a feature that was off (comments, actions) failed a Configure rebuild or a tenant open.
severity: significant
resolution: check now judges names against knownToolNames, every tool the package can register; TestKnownToolNames_MatchesRegistrations pins it to the newTool calls by AST. A real tool whose feature is off is accepted and filters nothing.
status: addressed
---
