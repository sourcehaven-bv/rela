---
id: RR-OTN4JU
type: review-response
title: Uses outside style blocks were not scanned
finding: Only .css files and .vue style blocks were checked for var() uses.
severity: minor
resolution: The use extractor now runs over the comment-stripped full text of every .vue, .css and .ts file.
status: addressed
---
