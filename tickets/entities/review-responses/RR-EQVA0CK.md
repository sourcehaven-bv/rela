---
id: RR-EQVA0CK
type: review-response
title: Identity bind reads the pool inside the create transaction
finding: The create hook bound system:flow inside the store transaction. On postgres the membership walk read on another pool connection while the transaction held its own and the write lock; under load this can exhaust the pool.
severity: significant
resolution: 'CreateHook.PrepareCreate binds system:flow before the transaction opens on CreateEntity, CreateLinked and CreateBatch; the header write reuses that Request. A when: reads only the subject''s own properties (flowcfg refuses related), so it reads no store. Test: TestCreateHook (prepared once, outside the transaction).'
status: addressed
---
