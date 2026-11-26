---
id: RR-Q7B99XW
type: review-response
title: Whitespace-only title hides the only identifier
finding: The untitled-row rule tested !title, so a title of only spaces counted as present and hide_id left the row with no visible identifier.
severity: minor
resolution: The rule now trims the title (EntityDetail showsId and MentionedByPanel); a test row with a blank title keeps its ID.
status: addressed
---
