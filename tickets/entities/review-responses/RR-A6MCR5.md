---
id: RR-A6MCR5
type: review-response
title: HasConfiguredScan nil-derefs on a nil metamodel, panicking the server at startup
finding: 'HasConfiguredScan walks p.m.Entities with no nil guard, so NewAttachmentPolicy(nil).HasConfiguredScan() panics. Independently reproduced: `runtime error: invalid memory address or nil pointer dereference`. NewAttachmentPolicy''s godoc says "m must be non-nil" but cannot enforce it — it returns a value, not an error — so the guard has to live in the method, exactly as ScanSockets() already guards p.m.Attachments. (A nil Attachments BLOCK is handled correctly; only a nil metamodel panics.) NewApp always passes a non-nil meta today, but that is a fact about the current call graph, not about the function, and this is a STARTUP DIAGNOSTIC — a warning intended to make a degraded state visible must never be able to take the server down instead. Project rule: constructors reject nil required fields, no silent fallbacks. Fix: `if p.m == nil { return false }` as the first line.'
severity: critical
resolution: 'Added `if p.m == nil { return false }` as the first statement of HasConfiguredScan, with a Nil: godoc clause explaining why a startup diagnostic must not be able to panic the server. Verified: the probe that previously produced `runtime error: invalid memory address or nil pointer dereference` now returns cleanly. Also added TestWarnIfScanCannotRun_NilMetamodel in internal/dataentry, which calls the warning with a nil metamodel and asserts it neither panics nor warns (a nil metamodel declares no scan). Note the guard is placed on HasConfiguredScan rather than NewAttachmentPolicy because the constructor returns a value, not an error, so it cannot reject nil — same shape as the existing ScanSockets() guard.'
status: addressed
---
