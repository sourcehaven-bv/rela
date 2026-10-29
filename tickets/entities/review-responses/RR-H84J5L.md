---
id: RR-H84J5L
type: review-response
title: ZWJ is allowed anywhere, so lookalike options get through
finding: 'internal/texthazard/texthazard.go:26 always allows U+200D, not only inside an emoji sequence. ''Yes'' and ''Ye‍s'' then render the same, pass the duplicate check, and post different values. The plan''s security section claims NFC plus the hazard list stops lookalike options, and an agent comparing the answer to its options misses the match. Fix: in normalizeListItem, allow U+200D only between two Extended_Pictographic runes (or after one, before an emoji modifier or another pictograph). Add negative cases ''Ye‍s'' and ''‍Yes'' to TestValidateProps_StringList.'
severity: minor
resolution: 'texthazard.Find now allows U+200D only between two emoji: the rune before must be a symbol (So) or a skin tone modifier, and the rune after must be a symbol. IsDisplayHazard reports U+200D as a hazard on its own. Go has no Extended_Pictographic table; So covers the pictographs that emoji sequences join. Added refusals for ''Ye‍s'', ''‍Yes'' and a trailing joiner to TestValidateProps_StringList, an accepted skin-tone ZWJ sequence, and Find table cases.'
status: addressed
---
