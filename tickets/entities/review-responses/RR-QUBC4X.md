---
id: RR-QUBC4X
type: review-response
title: Medium-width fields stack; the 10rem threshold is undocumented
finding: Fields narrower than 296px now stack, including span-4 at 1100px, and 10rem was an unexplained number.
severity: minor
resolution: 'Intended: a value column under 160px cannot hold a typical badge or picker, which is the overlap this bug reports. Threshold is now 160px with a comment giving the reason and the resulting 296px stacking width.'
status: addressed
---
