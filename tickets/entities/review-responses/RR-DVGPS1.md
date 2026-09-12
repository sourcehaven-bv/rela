---
id: RR-DVGPS1
type: review-response
title: Post-close recurrence files a new issue with no link to the closed one
finding: 'Dedup queries `--state open`, so a target that failed, was fixed and closed, then regressed gets a brand-new issue carrying no indication it has failed before. That is defensible as the dedup decision (a regression after a fix is genuinely new work), but it leaves a gap adjacent to the one this ticket set out to close: a post-close recurrence still reads like a first-time find.'
severity: minor
reason: 'Out of scope for this change, which is about splitting one thread into per-target threads. The fix is additive and self-contained — one extra `--state closed` query per target for the same exact title, and a "Previously filed and closed as #N" line in the body — so it can land later without revisiting anything here. Deferring also keeps the API call count at one per target for the first real sweep, which is the run that will exercise this code against the live API for the first time.'
status: deferred
---

Found by cranky-code-reviewer on the TKT-8LZGME diff (finding 6).

Worth doing, just not as part of the same change. Note the closed-issue query
doubles the per-target API calls, which interacts with the rate-limit failure
mode fixed in [[RR-LERUY1]] — better to land it once the current behaviour has
been observed against the real API.
