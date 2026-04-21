---
id: RR-Q9G5O
type: review-response
title: e2e test leaks metamodel assumption in search string
finding: '`search.fill(catBId.replace(''CAT-'', ''''))` assumes the prototype''s category id_prefix is CAT- forever. If the prefix or id_type changes, the dropdown fails to match. Fill with the full catBId instead; the RelationPicker''s fuzzy match handles substrings.'
severity: minor
resolution: Dropped the .replace('CAT-', '') — search.fill(catBId) now passes the full id. The RelationPicker already matches on substrings so no functional change, but the test no longer breaks if the prototype's category id_prefix is ever changed.
status: addressed
---
