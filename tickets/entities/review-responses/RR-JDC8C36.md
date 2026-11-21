---
id: RR-JDC8C36
type: review-response
title: 'Mounting a flow component started a schema load'
finding: 'read() called schema.load(), so any component using useFlowList started a full schema load when the store was not marked loaded. Four EntityDetail tests that seed the store by hand failed, and git.ts has no such side effect.'
severity: significant
resolution: 'useFlowList no longer calls load(). It waits for schema.loaded in a detached effect scope and then decides on flowsEnabled, as git.ts reads gitEnabled. The four EntityDetail test files pass again, and a test asserts load() is not called.'
status: addressed
---
