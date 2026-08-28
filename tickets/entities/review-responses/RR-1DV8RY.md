---
id: RR-1DV8RY
type: review-response
title: 'Caching and the disk-cache key are unaddressed: an elevated render is principal-independent, which is both an opportunity and a poisoning risk'
finding: |-
    The ticket says nothing about caching, but elevation changes the caching contract in a way that cuts both ways.

    A normal document render is principal-DEPENDENT: two principals legitimately get different bytes, so any shared cache keyed on doc name alone would serve one principal's view to another. document.go:111 notes script: renders bypass the disk cache on both sides, which sidesteps this today.

    An ELEVATED render is principal-INDEPENDENT by construction — it reads the same rows regardless of caller. That makes it the one document kind that is genuinely safe to cache under a shared key, and also the most expensive kind (a company-wide aggregate over four entity types), so caching is exactly what it wants. Worth stating as a deliberate follow-up rather than leaving the reader to infer it.

    The risk is the mirror image: if elevation ever becomes conditional (e.g. a script that elevates only on some branch, or a future partial-elevation mode), a cache keyed on doc name would let an elevated render populate an entry later served to a principal who should have received the gated view. Any caching work must therefore key on the elevation posture, not just the name, and the ticket should say so before someone adds caching later without this context.

    Related existing state: RR-P4E9GL (deferred) already notes standalone renders have neither dedup nor a concurrency cap, and TKT-OGR566 (backlog) covers bounding concurrent Lua document renders. An elevated company-wide aggregate is precisely the expensive render those tickets are about, so this ticket should reference them rather than rediscover the problem.
resolution: |
  Deferred: caching is out of scope for this ticket, and the constraint is recorded
  for whoever adds it.
  
  The observation stands and is now written into the plan's risk table: an elevated
  render is principal-INDEPENDENT by construction, which makes it both the most
  worthwhile document kind to cache and the most dangerous to cache carelessly. Any
  future cache MUST key on the elevation posture, not merely the document name --
  otherwise an elevated render could populate an entry later served to a principal
  who should have received the gated view.
  
  Existing related work: TKT-OGR566 (bound concurrent Lua document renders) and
  RR-P4E9GL (standalone renders have neither dedup nor a concurrency cap, deferred).
  An elevated company-wide aggregate is precisely the expensive render those cover.
  
  Note also TKT-PX5YL7: document scripts currently hold write bindings, so renders
  are not provably side-effect-free today; that is a prerequisite for treating a
  render as cacheable at all.
reason: 'Deferred to whoever adds render caching: this ticket adds no cache, so there is nothing to key incorrectly yet. Deferring is safe because the hazard is latent rather than live -- script: renders bypass the disk cache today (document.go:111). Two prerequisites are also outstanding: TKT-PX5YL7 (document scripts still hold write bindings, so a render is not provably side-effect-free and therefore not safely cacheable at all) and TKT-OGR566 / RR-P4E9GL (no dedup or concurrency cap). The constraint -- any cache MUST key on elevation posture, not merely document name -- is recorded in PLAN-1DETM0''s risk table so it is not rediscovered later.'
severity: minor
status: deferred
---
