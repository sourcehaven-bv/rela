---
id: RR-U4H8BQ
type: review-response
title: Identity conjuncts are never pushed, so their derived index is dead weight
finding: 'appbuild''s Resolve calls ConditionPrefilters with a hardcoded empty identity, so every current_user conjunct is skipped and never lowered into the store query. That is SOUND — it under-pushes, so the pushdown stays a strict superset and applyScope remains authoritative, which both reviewers confirmed by tracing. But AC11''s listScopeIndexProperties derives index columns for exactly those conjuncts, so `mijn:` gets an index built for a predicate this path guarantees will never be pushed: dead weight plus a sequential scan, the precise drift ConditionIndexProperties'' own doc warns about.'
severity: minor
reason: 'Deferred because closing it means resolving the identity per request and threading it into Resolve, and the safe shape matters: the resolver is built per (cfg, meta) and is NOT request-scoped, so an identity stored on the struct would be shared across principals — principal A''s list request could leave A''s identity in a query served to B. That is a correctness-critical change to make deliberately, with its own test, not as a review tidy-up. The current behaviour is slow, not wrong.'
status: deferred
---

The empty-identity literal is now commented at the call site with what it buys,
so the next person threading an identity through sees why it was empty rather
than assuming it was an oversight.

Two ways out, both fine: push the identity in per request (and delete nothing),
or stop deriving index columns for identity conjuncts until it is pushed. The
second is smaller and leaves no dead index; the first is what makes `mijn:`
actually fast, which AC11 calls the sharp case.
