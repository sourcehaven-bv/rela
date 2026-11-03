---
id: RR-CUBN6B
type: review-response
title: D3 implicit promote is gated by post roles and shares the single action slot
finding: Gating the implicit promote by mt.Post lets anyone who may post that type promote anyone's message, with no Denied text or permission, and a non-promoting action such as acknowledge permanently blocks promote. Prefer requiring an explicit action that carries promote_to (refuse a bare message-level promote_to), or document the one-slot limitation in docs/realtime.md.
severity: minor
resolution: 'Per Jeroen D3: promote needs an explicit action with its own gate. A bare message-level promote_to is refused at load and there is no implicit promote action.'
status: addressed
---
