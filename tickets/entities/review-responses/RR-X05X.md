---
id: RR-X05X
type: review-response
title: 'C2/C3: RelationFilePath panic on `..` substring crashes process on legitimate names'
finding: internal/project/context.go contained a defensive `mustBeSafePathComponent` that panicked on any string containing `..`, including `a..b` or future relation type names like `v1..v2`. Worse, RelationFilePath is called from non-HTTP contexts (sync, file watcher, repository writes), so a panic crashes the goroutine or even the process at startup. The reviewer correctly identified this as both incorrect (rejects valid input) and dangerous (panic on user-controlled YAML).
severity: critical
resolution: Removed the defensive panic from RelationFilePath. The actual choke points are model.ValidateID at entity creation and metamodel.ValidateRelation in workspace.CreateRelation, which already block path-traversal inputs before they reach the path constructor. Documented the invariant in a comment so the next defensive check (if needed) lives at the new untrusted entry point. Deleted relation_path_security_test.go since it tested the panic that no longer exists.
status: addressed
---
