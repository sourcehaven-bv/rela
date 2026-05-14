---
id: RR-W8ZR
type: review-response
title: 'Cranky #11: relation-delete error swallowing'
finding: Manager.DeleteEntity and cascadeHost.DeleteEntity silently swallow non-NotFound relation-delete errors — graph can corrupt with no caller-visible error.
severity: significant
reason: Pre-existing bug shipped in TKT-QTNX (not introduced by TKT-IU2S). Filed as BUG-C20T.
status: deferred
---
