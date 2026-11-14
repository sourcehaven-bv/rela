---
id: RR-VSMLC99
type: review-response
title: The rela flow Run logic has no tests
finding: Every new test called write() directly; nothing pinned how show and validate combine headless flows with a broken or missing flows.yaml.
severity: significant
resolution: TestFlowShow_Run and TestFlowValidate_Run drive Run over real services with a valid, missing and broken flows.yaml, in text and JSON.
status: addressed
---
