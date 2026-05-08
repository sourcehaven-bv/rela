---
id: RR-W8BRB
type: review-response
title: Multi-block list continuation emitted lines with trailing whitespace
finding: Empty continuation lines in multi-block list items got `  ` (two-space indent), which (a) trips MD009 lint and (b) is the CommonMark hard-break marker, risking misinterpretation.
severity: significant
resolution: renderListItem and prefixLines now emit empty continuation lines with no indent (or, for blockquote, the trimmed `>` prefix). prefixLines comment explains the policy.
status: addressed
---
