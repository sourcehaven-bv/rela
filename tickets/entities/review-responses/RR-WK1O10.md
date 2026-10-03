---
id: RR-WK1O10
type: review-response
title: Frontmatter parser drops globs after blank or comment lines
finding: A blank or comment line ended the paths list and inline comments stayed in the glob.
severity: minor
resolution: Blank and comment lines are skipped; listItem strips quotes and trailing comments; flow-style paths fail with a clear message; TestListItem.
status: addressed
---
