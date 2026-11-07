---
id: RR-5909IGN
type: review-response
title: Roll-up properties in enforced rules
finding: A rule on a roll-up would always see nil on the write path.
severity: minor
resolution: The predicate environment already refuses roll-up attributes; TestCompile_RefusesRollup pins it.
status: addressed
---
