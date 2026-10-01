---
id: RR-2DYR8I
type: review-response
title: Oversized classification.yaml aborts acl audit
finding: A read error (e.g. over the size limit) failed the whole audit, and the parse warning went to stdout, corrupting -o json.
severity: minor
resolution: Any read or parse problem is a warning on stderr and the classification findings are skipped. Test covers broken and oversized files and checks the report stays clean.
status: addressed
---
