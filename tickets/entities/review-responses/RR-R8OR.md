---
id: RR-R8OR
type: review-response
title: Dry-run shares writeMu with real writes; lock-contention + serialization risk
finding: 'The plan reuses the create handler with a ?dry_run early-return. But handleV1CreateEntity takes a.writeMu.Lock() at the top (api_v1.go:448), and the autosave-style live re-derivation fires a request per debounced edit. If dry-run keeps the write lock, every keystroke-debounce serializes against real writes from other users and against itself, turning a read-only affordance check into a writer-lock contender. The dry-run path must NOT acquire the write mutex (it persists nothing) and must snapshot state once (a.State()) like a read handler. Decide and document: dry-run is a READ-shaped operation. Otherwise live validation degrades server throughput under concurrent editing.'
severity: significant
resolution: 'Plan updated: the dry-run early-return happens BEFORE a.writeMu.Lock(); dry-run snapshots a.State() once like a GET and never takes the writer mutex (it persists nothing). Live re-derivation per debounced edit therefore does not contend the writer lock.'
status: addressed
---
