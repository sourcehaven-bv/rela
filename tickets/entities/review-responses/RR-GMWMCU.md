---
id: RR-GMWMCU
type: review-response
title: AC 7's claim about bob's wake is vacuous; prove it in AC 6
finding: In AC 7, alice and bob both see TKT-1 and TKT-2, so their wakes are identical. "Bob's wake never holds an anchor his page does not show" is then trivially true. Move that assertion to AC 6's "hidden row" subtest, where the pages differ. Read bob's liveSub.busyWake through a test accessor under c.mu, as hashOf does (live_test.go:869-880), and assert that it has no TKT-2 key while alice's has one.
severity: minor
resolution: 'Adopted: the vacuous sentence is removed from AC 7, and AC 6 "hidden row" asserts through a busyWakeOf accessor under c.mu that bob''s wake has no TKT-2 key while alice''s has one.'
status: addressed
---
