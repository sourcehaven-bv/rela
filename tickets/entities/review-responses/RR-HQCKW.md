---
id: RR-HQCKW
type: review-response
title: Two FS handles into same .rela/ under different crypto regimes
finding: Plan keeps bytes StoreFS (decorated) and fs storage.FS (raw). Index is routed through bytes (sealed). cleanupTempFiles uses raw fs to remove .new files (already-sealed) — coherent, but plan never states the invariant explicitly. Next maintainer who adds an 'easy' raw-read in fsstore will silently bypass decryption.
severity: significant
resolution: Plan introduces DirFS as a separate, structurally narrower interface for the raw handle. DirFS deliberately omits ReadFile/WriteFile/Open so the compiler enforces that no data bytes bypass the StoreFS transform stack. The invariant is now enforced at the type level instead of by prose convention.
status: addressed
---
