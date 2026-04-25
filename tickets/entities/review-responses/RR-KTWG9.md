---
id: RR-KTWG9
type: review-response
title: validateDisplayProperty bails on first error instead of accumulating
finding: 'validateDisplayProperty returns early on whitespace, hiding existence problems. If the author types display_property: '' nonexistent '', they get the whitespace error, fix it, re-run, then get the missing-property error. Two round-trips. Cheap to do both: collect into []string and return all of them. Matches the rest of the validator''s accumulating style and lets future fields (display_format, display_template) extend without restructuring.'
severity: minor
resolution: Restructured validateDisplayProperty around an accumulating []string. Whitespace-and-existence still bail-then-return (the deeper checks would error on a value whose user-meant form we can't be sure of), but the list-typed and disallowed-type checks now both append and return together. Doc comment notes the accumulating contract for future fields like display_format.
status: addressed
---
