---
id: RR-X7EFU9D
type: review-response
title: Handler tests miss non-mutable type and CSRF
finding: The append-only case tested an unknown type and PATCH/DELETE had no CSRF test.
severity: minor
resolution: Added a declared append-only type case (400) and PATCH/DELETE to TestRecords_CrossOriginRefused.
status: addressed
---
