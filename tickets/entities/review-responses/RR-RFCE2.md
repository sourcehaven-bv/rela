---
id: RR-RFCE2
type: review-response
title: Factory eager keyring load silently empties prop cache for writer-only callers
finding: factory.go:50-77 calls loadEncryption -> LoadFromDir which silently allows nil identity. cryptofs.New(...kr.Identity()) constructed with possibly-nil identity. Writer-only caller (CI sealing files, automation) hits ErrNoPrivateKey on any read including loadPersistedIndex and rebuildPropCache. Both swallow errors (index.go:46-48, 160-163) — store opens but produces silently-empty prop cache. Encrypt-only workflows silently broken.
severity: significant
resolution: FSFactory.OpenStore now returns ErrEncryptedRepoNeedsIdentity when wantSealed=true but kr.Identity() is nil. Writer-only callers who would previously have silently opened a store with empty prop cache now get a clear, actionable error at factory time. TestFSFactory_EncryptedNeedsIdentity locks the behavior in.
status: addressed
---
