---
id: RR-ZI5ZLB
type: review-response
title: 'PR-B: pg notifyPut lacked the observer skip - three backends, two rules'
finding: 'pgstore''s notifyPut had no non-default-state guard while fs/mem both carry it — an observer registered via the exported pgstore.WithObserver (documented as ''matches memstore.WithObserver'') would receive state puts and corrupt bare-id-keyed search documents. Latent only because pg''s own search observer is currently a no-op stub; the reviewer proved the divergence live ([PAGE-1 PAGE-1@draft] vs [PAGE-1]). The storetest suite could not catch it: it asserts Subscribe() events (deliberately per-state), never observer fan-out.'
severity: critical
resolution: Identical three-line guard + comment added to pgstore.notifyPut; new DB-gated TestObservers_SkipNonDefaultStates in pgstore mirrors the fs/mem pins so all three backends now enforce and test one rule.
status: addressed
---
