---
id: RR-HITIQO
type: review-response
title: 'Nits: blank line, z-index token, wording, exact role match, play-test scope'
finding: Stray blank line in EntityBody; literal z-index 1 instead of --rl-z-sticky; 'a click places nothing' / 'place a caret' wording; getByRole substring match; play test claims more than it checks and clicks instead of using the keyboard.
severity: nit
resolution: 'Fixed all: blank line removed, var(--rl-z-sticky), wording rewritten, exact: true, play comment narrowed and AC4 now uses Escape then Enter from the focused button. afterEach cleanup not added: each test gets its own backend.'
status: addressed
---
