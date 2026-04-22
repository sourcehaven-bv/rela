---
id: RR-UQ225
type: review-response
title: forms 'blocks submission or shows error' OR-assertion masks regressions
finding: expect(hasError || stillOnForm).toBeTruthy() reduces to 'we didn't navigate' because submit() swallows the waitForURL timeout. Passes even if the form hangs.
severity: critical
resolution: 'Rewrote to observe network: attaches a response listener, asserts no POST fires AND we''re still on /form/feature. Removes the vacuous OR-assertion.'
status: addressed
---
