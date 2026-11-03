---
id: RR-276BRT
type: review-response
title: Library story documents an app config key
finding: 'frontend/packages/rela-components/src/components/channel/RlChannelDecisions.stories.ts:78 says ''as an operator marks it with style: primary''. rela-components is a standalone library and does not know about realtime.yaml. The types.ts change in the same diff removed exactly this kind of ''why the app sets it'' wording. Fix: ''The primary decision of the row; the others stay secondary.'''
severity: nit
resolution: Story comment is now 'The primary decision of the row; the others stay secondary.'
status: addressed
---
