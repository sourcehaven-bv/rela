---
id: RR-50NBNG
type: review-response
title: Deferring per-value buttons means the PoC gains nothing in the web app yet
finding: 'The ticket''s motivation is the PoC''s approve/revise. Today `approve` is one click (optional note) and `revise` opens a form. Under this plan a `decide` enum type is always a form with a select and, per the finding above, has no expect_generation. The deferral (Q4) is reasonable because deriving presets from guards is brittle. Still, file the authored-preset follow-up now (button: {label, props}, key "<type>:<to>") and link it from this ticket, and say in docs/realtime.md that two message types remain the better choice when you want one-click buttons.'
severity: minor
resolution: Filed the authored-preset follow-up as a proposed ticket in Q4 and added a docs sentence that two message types remain better for one-click buttons.
status: addressed
---
