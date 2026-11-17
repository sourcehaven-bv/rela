---
id: RR-HN6I1U7
type: review-response
title: Go matcher and store comparison disagree
finding: A strict Go date parse denies values the byte-wise store comparison admits; fs can hold time.Time values.
severity: significant
resolution: 'Plan: one shared byte-wise matcher in store; time.Time renders as YYYY-MM-DD; when: properties validated as hard errors on write.'
status: addressed
---
