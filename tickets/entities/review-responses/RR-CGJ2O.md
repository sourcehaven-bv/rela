---
id: RR-CGJ2O
type: review-response
title: No concurrency test for delete merge path
finding: 'Two concurrent update_entity calls deleting different keys: the store has its own locking on update so it should be fine, but no race-detector test covers the new merge path. -race already runs in just test; one explicit test would document the contract.'
severity: nit
reason: The merge path operates on the entity's local Properties map (already cloned by the store on GetEntity) and the workspace's UpdateEntity owns concurrency control. `just test` already runs with `-race`, so any latent data race in the merge would surface there. A dedicated concurrency test would only re-prove what the existing race-detector run already covers, with little signal value. Skipping.
status: wont-fix
---
