---
id: RR-INNN5B
type: review-response
title: Plain text related values had no separator
finding: Text values rendered side by side with only a gap, and inline-flex prevented ellipsis.
severity: minor
resolution: Preformatted kinds carry no widget; when no value has a widget the field renders the comma-joined text with ellipsis. related-values also clips overflow.
status: addressed
---
