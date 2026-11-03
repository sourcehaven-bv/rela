---
id: RR-5NE8H03
type: review-response
title: End block narrowed by inclusion drops types the schema does not know yet
finding: With two blocks the end block got the attached end types, so a type the server lists but the held channel schema does not know yet (between two refresh reads) showed in neither block.
severity: minor
resolution: 'ChannelsSection takes an exclude prop; the end block excludes the types placed above instead of naming its own. Test: shows a type the held schema does not know in the end block.'
status: addressed
---
