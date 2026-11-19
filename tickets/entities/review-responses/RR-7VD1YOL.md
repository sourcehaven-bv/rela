---
id: RR-7VD1YOL
type: review-response
title: No test for a template title with one hidden placeholder
finding: TestFlowControl_SubjectTitleHidden used only a single display property and only RunList; a templated title with one hidden placeholder is where a partial title leaks.
severity: significant
resolution: The test now runs a single property and a template {name} {salary} with salary hidden through RunList and Run and Tasks. It fails with plain DisplayTitle.
status: addressed
---
