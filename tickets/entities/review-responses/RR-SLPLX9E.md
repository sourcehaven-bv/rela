---
id: RR-SLPLX9E
type: review-response
title: 'Code: List gives the caller no resume point'
finding: Rows dropped at load make List return fewer or zero elements while the range has more; a caller reading that as exhausted would skip the rest of the range.
severity: significant
resolution: 'List returns ElementPage{Elements; Last; More}: Last is the last listed address kept or not and More says the listing stopped at the limit. Tested in TestElements_DropsRowGoneAtLoad.'
status: addressed
---
