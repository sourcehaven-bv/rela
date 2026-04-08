---
id: RR-E4HZ
type: review-response
title: _= w.automation.Swap(nil) is awkward; automation engine is never closed
finding: The discarded Swap is noise. Use w.automation.Store((*automation.Engine)(nil)) for clarity. Separately, the old automation engine is never closed — fine today because Engine has no resources, but worth a comment.
severity: nit
resolution: Resolved as a side effect of the workspaceState bundling. The automation field is no longer a standalone atomic.Pointer that needs Store(nil); it's just a field inside workspaceState and is set to nil by the new-state builder when the metamodel has no automations. The `_ = w.automation.Swap(nil)` line is gone.
status: addressed
---
