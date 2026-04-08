---
id: RR-8M2G
type: review-response
title: No backend test for percent-encoded brackets in filter URLs
finding: Vue Router URL-encodes brackets to %5B/%5D. The plan asserts Go decodes before HasPrefix check but provides no test reference. Existing tests use literal brackets. Add a test exercising filter%5Bstatus%5D=open and filter%5Bstatus%5D%5Beq%5D=open to catch any future stdlib breakage. 10 minutes of work.
severity: significant
resolution: Added api_v1_test.go cases for filter%5Bstatus%5D=open, filter%5Bdue_date%5D%5Blte%5D=$today, and repeated multi-value via filter%5Btags%5D%5Bin%5D%5B%5D=a&...=b.
status: addressed
---
