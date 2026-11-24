---
id: RR-D01C1JD
type: review-response
title: Postgres paging not exercised
finding: The paging differential test was only run on the default, memory and sqlite tags.
severity: minor
resolution: Ran the dataentry search tests with -tags postgres against a local PostgreSQL; all differential subtests pass.
status: addressed
---
