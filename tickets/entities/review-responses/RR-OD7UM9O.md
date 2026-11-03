---
id: RR-OD7UM9O
type: review-response
title: Validate and server error headers differ in file path
finding: CheckDataEntry uses the relative ConfigFile while the server joins the root, so the two headers differ; the comment claimed the same error.
severity: nit
resolution: 'Comment now says the same error type. The relative name is deliberate: CheckDataEntry has no root.'
status: addressed
---
