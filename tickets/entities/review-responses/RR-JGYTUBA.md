---
id: RR-JGYTUBA
type: review-response
title: buttonTypes stays held if the head refetch fails
finding: 'useSessionDecisions buttonTypes: the hold ends only on a successful takeHead.'
severity: minor
resolution: DecisionChannel gained an optional error ref (useChannel's); a read error ends the hold. Unit test.
status: addressed
---
