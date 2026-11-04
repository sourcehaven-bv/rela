---
id: RR-6VCPH64
type: review-response
title: CSRF exemption doc and test for _batch
finding: isNonBrowserExemptV1Path documents that underscore paths stay browser-gated; _batch needs an explicit match, a doc update and a cookie/Origin test.
severity: nit
resolution: batchPath is in nonBrowserExemptPrefixes with a comment; docs/acl-security.md has a Batch create section; TestBatch_CrossOriginRefused covers cookie, Origin and script requests.
status: addressed
---
