---
id: RR-MJJM7Q
type: review-response
title: Restore fails when a child's delete grant comes through its owner
finding: Each owned child was authorized under WithRevealed of its own family only, so the owner->child edge and the marked owner were invisible and the undo returned 403.
severity: significant
resolution: store.WithRevealed takes several ids on all four backends; restore authorizes owner and children with every family revealed; markedSource also overrides ListEntities. TestRestore_OwnedGrantThroughOwner and storetest RevealSeveralMarks.
status: addressed
---
