---
id: RR-2OT3VK
type: review-response
title: Module threads cost about 1 MB each and nesting is unbounded
finding: NewThread copies the parent's 64K registry and fixed call stack, and a thread per module escapes the call-stack limit; a 400-deep chain used 448 MB.
severity: significant
resolution: 'newModuleThread starts each thread with a 1024-slot growable registry and an auto-growing call stack bounded by the parent''s limits (30 modules: about 1.9 MB total); nesting is capped at maxModuleDepth=32. Pinned by TestRequire_DeepChainIsCapped, TestRequire_ChainAtDepthLimitLoads and TestRequire_ModuleThreadGrowsToRuntimeLimits.'
status: addressed
---
