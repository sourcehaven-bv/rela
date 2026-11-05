---
id: RR-FICTL99
type: review-response
title: Relation files with unparseable sides vanished from the list
finding: Relations were detected from the parsed sides, so a relation file whose sides both failed to parse fell to the entity branch and was hidden from everyone.
severity: significant
resolution: Relation files are recognised by location and their name; the file-name edge is always gated.
status: addressed
---
