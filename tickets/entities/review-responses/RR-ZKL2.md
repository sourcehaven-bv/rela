---
id: RR-ZKL2
type: review-response
title: 'Debounce + stale-drop races: ordering guarantee for live verdict responses'
finding: 'Plan says ''mirror autosave''s AbortController stale-drop.'' Make explicit: the form must apply ONLY the verdict response matching the latest request (monotonic request token or AbortController per the existing autosave pattern), else an out-of-order earlier response could re-enable a field the newer state disables — a visual flip-flop and a brief window where a denied input looks editable. Autosave already solves this (useAutoSave.ts AbortController); confirm the create path reuses the SAME mechanism rather than a parallel one. Edge case to test: type A then quickly type B; only B''s verdicts apply.'
severity: minor
resolution: 'Plan: create path reuses the SAME AbortController stale-drop mechanism as useAutoSave.ts; only the latest request''s verdicts apply. Test: type A then B fast -> only B applies.'
status: addressed
---
