---
id: RR-AWR6K8
type: review-response
title: Include failure log implied one entity
finding: The page-wide include error logged entity=<first row>, which reads as if one row failed while the whole page lost its include block.
severity: minor
resolution: Message now says the block is omitted for every row; the attribute is first_entity.
status: addressed
---
