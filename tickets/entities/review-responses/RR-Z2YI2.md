---
id: RR-Z2YI2
type: review-response
title: Factory's OnPostWrite subscribe is silent type-cast; broken if FS isn't SafeFS
finding: 'factory.go:73-75 has `if safe, ok := f.FS.(*storage.SafeFS); ok { safe.OnPostWrite(s.RecordWrite) }`. If caller constructs FSFactory{FS: storage.NewOsFS()} (no SafeFS wrap), cast fails silently, observer never installed, watcher self-echo permanently wrong for encrypted repos — every internal write becomes external event, reconcile-on-self loop. No compile-time or run-time assertion prevents this. factory_test.go and mcp/watcher_test.go already construct FSFactory with raw OsFS without issue only because they don''t start the watcher.'
severity: significant
resolution: FSFactory.OpenStore now returns ErrEncryptedRepoNeedsSafeFS when wantSealed=true but the FS isn't a *storage.SafeFS. Silent cast removed. TestFSFactory_EncryptedNeedsSafeFS asserts the fail-loud behavior. Cleartext repos are unaffected (SafeFS still optional for backwards compatibility of non-encryption-dependent test setups).
status: addressed
---
