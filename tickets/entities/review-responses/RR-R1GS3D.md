---
id: RR-R1GS3D
type: review-response
title: The fold button may be smaller than the 24 px minimum target
finding: 'RlChannelFold.vue:55,61 sets padding: 2px with font-size xs, so the button is about 22 px tall. That is under the WCAG 2.2 SC 2.5.8 minimum of 24 by 24 px. The button is the only way to expand a run. Fix: add min-block-size: 24px (or more padding) and assert the height in the RlChannelFold story.'
severity: minor
resolution: 'Fixed. RlChannelFold sets min-block-size: 24px; the Collapsed story asserts the button is at least 24 px tall.'
status: addressed
---
