---
id: rename-no-overwrite-test
type: automated-measure
title: 'Test: rename onto an occupied ID never overwrites the target (atomic store.RenameEntity)'
description: Regression for BUG-5QDV6F. After the fix re-routed Manager.RenameEntity through the atomic store.RenameEntity (deleting the non-atomic internal/rename decomposition), TestRename_TargetExistsDoesNotOverwrite asserts a rename onto an ID another entity occupies returns ErrEntityAlreadyExists, performs ZERO creates/updates/deletes (counting store), and leaves the target's title untouched — proving no clobber. Backed by the storetest conformance suite which pins store.RenameEntity's not-found/conflict/self-ref/relation-rewrite behaviour across mem/fs/pg. Fails if rename reintroduces a non-atomic create-then-update path or stops surfacing the conflict.
kind: test
location: internal/entitymanager/manager_test.go (TestRename_TargetExistsDoesNotOverwrite) + internal/store/storetest (RenameEntity conformance)
status: active
---
