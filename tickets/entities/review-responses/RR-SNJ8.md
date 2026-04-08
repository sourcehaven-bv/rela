---
id: RR-SNJ8
type: review-response
title: Client interface too narrow; embeddings will force breaking changes to Lua plumbing
finding: 'Plan rationalizes embeddings as ''package shape supports this'' but the *Lua runtime field* is aiClient ai.Client. When embeddings land, you either add a parallel field (aiEmbedder) and a parallel option (WithAIEmbedder) — two wirings to keep in sync — or you widen the Client interface and break every test double. Fix: introduce ai.Provider (or ai.Service) aggregate now with only Chat() defined. Lua takes WithAIProvider. Embeddings just add a method to the implementation. One wiring point forever.'
severity: significant
resolution: Renamed Client to Provider throughout. Lua runtime field is aiProvider ai.Provider, option is WithAIProvider. When embeddings land, the Provider interface widens with Embed(...) and only test fakes need a stub method — no parallel wiring. Documented the rationale and the alternative (Chatter+Embedder small interfaces) in the plan.
status: addressed
---
