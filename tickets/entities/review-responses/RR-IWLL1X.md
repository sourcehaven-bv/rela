---
id: RR-IWLL1X
type: review-response
title: Migration header wording
finding: '"A new empty table takes no lock on existing ones" is not quite true. The REFERENCES channels(id) constraint takes a SHARE ROW EXCLUSIVE lock on channels for the length of the CREATE. It is brief, but say so in the 0036 header in the style of 0035.'
severity: nit
resolution: 0036 header will state the brief SHARE ROW EXCLUSIVE lock on channels taken by the REFERENCES constraint.
status: addressed
---
