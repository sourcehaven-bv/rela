---
id: RR-JQUPMZP
type: review-response
title: Session property test cannot fail as a property
finding: 'Assertions run inside land within unawaited reads, so violations are unhandled rejections: fast-check records a pass, shrinks nothing and prints no counterexample.'
severity: significant
resolution: Violations are collected, commands awaited, checks run at the end. Disabling each session rule now fails with a counterexample.
status: addressed
---
