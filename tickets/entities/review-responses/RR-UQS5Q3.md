---
id: RR-UQS5Q3
type: review-response
title: The risk names fsimport, but the affected path is the sqlite-to-markdown export
finding: 'fsimport writes into a database target; fsstore is only its read-only source, so fsimport is unaffected. The direct fsstore writer for external data is appbuild/projectdata_sqlite.go: export data, then copyData(src sqlite, dst fsstore). After this change, a database row whose type the schema no longer declares fails partway through the export, which leaves a partial target. Add a preflight in copyData that lists the undeclared types before the first write, and test it. Correct the risk text.'
severity: minor
resolution: 'Plan updated: the risk names the sqlite-to-markdown export (copyData), which gets a preflight listing undeclared types before the first write (AC8); fsimport is unaffected.'
status: addressed
---
