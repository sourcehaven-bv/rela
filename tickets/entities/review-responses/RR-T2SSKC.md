---
id: RR-T2SSKC
type: review-response
title: Configure picker and channel chips used the old colour list
finding: OPTION_COLORS lacked teal; tagColor mapped yellow to amber, kept cyan and drew shades and operator colours grey; channel enum chips ignored kit classes.
severity: significant
resolution: RlTag gained yellow and teal; tagColor maps each hue to its own tag and a shade to its hue; picker offers teal; channel chips map kit-<hue>-<shade> to the hue; names.test.ts.
status: addressed
---
