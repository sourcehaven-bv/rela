---
id: RR-CN1DY8
type: review-response
title: The rela HTTP wire types do not belong in rela-components
finding: '§0 "The wire is the model" puts a mirror of `apiwire/v1/channels.go` in the library and says "as api/comments.ts does". It does not: api/comments.ts imports `@/types/entity`, and the library''s comment/types.ts is presentational ("the library does not know what an anchor refers to"). The library is the Atlas Projects component set. Binding its props to rela''s HTTP shapes (`_actions`, `next_after_rev`, `props: map[string]any`) means every wire change is a library change. Fix: keep the wire mirror in `frontend/src/types/channels.ts`. The library declares small presentational types (message row, author, ref chip, field descriptor, affordance flags) that the wire maps onto in `ChannelsPanel`/`useChannel`. That also keeps the library stories independent of a Go golden.'
severity: significant
resolution: The wire mirror moves to frontend/src/types/channels.ts and the library declares its own small presentational types that the app maps onto (R10).
status: addressed
---
