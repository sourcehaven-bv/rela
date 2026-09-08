---
id: RR-VFWE4C
type: review-response
title: CloseAssembly claimed goroutine-safety it did not have; reload held the mutex across assembly
finding: 'stopBackgroundServices nils mailStop/gcStop/jobQueue without a lock while Services.Jobs() reads jobQueue unlocked, so the documented ''safe from multiple goroutines'' invited a racy use. Separately, reload() held s.mu across the whole Assemble (disk I/O, Lua compilation, on postgres a network round-trip), blocking Deps() and Close(). Two minor consistency issues: watchSchema took the mutex twice for two related reads, and stopSchemaWatch was assigned outside it.'
severity: minor
resolution: Narrowed the CloseAssembly doc to what actually holds (safe repeatedly and against Close; not against a concurrent Jobs()). reload() now builds the successor outside the lock and takes it only for the swap, re-checking for supersession. watchSchema captures one snapshot and assigns stopSchemaWatch under the mutex.
status: addressed
---
