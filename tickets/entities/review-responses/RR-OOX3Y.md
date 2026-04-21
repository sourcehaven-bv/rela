---
id: RR-OOX3Y
type: review-response
title: Walk test asserts sorted-set equality; doesn't verify traversal order
finding: TestRootedFS_Walk_ReturnsKeys uses sort.Strings before comparison, which hides the fact that the test does not verify the preorder-depth-first contract of filepath.WalkDir.
severity: minor
reason: RootedFS.Walk delegates to the underlying FS.Walk and promises nothing about order (the doc comment does not mention ordering). Asserting traversal order would lock the test to a specific FS implementation behavior that RootedFS does not guarantee. Test name clarified via comment; unordered-set semantics are correct for what RootedFS actually promises.
status: wont-fix
---
