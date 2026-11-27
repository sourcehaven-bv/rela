---
id: RR-2Y3O3SE
type: review-response
title: Lookup error path (RR-L6LJZQS) has no test
finding: 'No test injects a ResolveIDsErr failure. A refactor to servedIDs would turn an id into a title and make ne admit every row without failing a test. Fix: a failing-store test asserting 500 for eq and ne.'
severity: significant
resolution: 'TestRelationFilter_LookupFaultFailsTheList covers a gate fault and a header-read fault for eq and ne: gateAppListQuery returns errListLoad and the handler answers 500.'
status: addressed
---
