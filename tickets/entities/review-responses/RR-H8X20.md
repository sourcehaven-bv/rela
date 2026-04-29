---
id: RR-H8X20
type: review-response
title: Returned q ref is mutable, no setter guard
finding: useUrlFilterSync returns q as a mutable ref. If a different actor sets q.value directly, the URL would lag behind. Today this does not happen because all q writes go through writeToQuery, but the invariant is implicit and there is no test.
severity: significant
resolution: useUrlFilterSync now returns q wrapped in readonly() with type Readonly<Ref<string>>. The only sanctioned write path is writeToQuery, which keeps URL and local mirror in lockstep. Direct q.value mutations now produce a TypeScript error.
status: addressed
---
