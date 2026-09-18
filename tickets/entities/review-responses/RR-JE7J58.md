---
id: RR-JE7J58
type: review-response
title: scopeScalesToEditor regex transform was undocumented and unasserted
finding: 'The build rewrites scales.css with two regexes so the spacing and radius tokens do not land
  on the app''s :root as an unpromised contract. The reasoning is sound but appeared nowhere in the plan,
  and nothing checked the transform matched. Reformat scales.css so :root no longer starts a line (wrap
  it in @layer, say) and it silently stops matching: every scale token leaks onto the app''s :root and
  nothing fails.'
severity: minor
resolution: The transform now counts its substitutions and throws when fewer than two match, with a message
  naming the consequence. Verified by indenting :root in scales.css and watching the build fail. This
  mirrors the @import guard already in the same plugin.
status: addressed
---
