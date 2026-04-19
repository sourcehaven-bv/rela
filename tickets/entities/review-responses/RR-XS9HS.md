---
id: RR-XS9HS
type: review-response
title: SafeFS-pins-to-OsFS test-mode blast radius not measured
finding: Plan acknowledges SafeFS.WriteFile uses os.OpenFile directly (safefs.go:40) but doesn't say HOW to test the new EncryptedFS decorator. MemFS without SafeFS doesn't exercise atomic-temp-rename interaction; SafeFS(MemFS) writes to OS. Test layering needs to be explicit so reviewers don't argue about it.
severity: minor
resolution: 'Plan explicitly states test layering: EncryptedFS unit tests use raw MemFS (no SafeFS). Full-stack fsstore tests use EncryptedFS(SafeFS(OsFS)) against t.TempDir(). Plan also documents why SafeFS still owns its whole write (it owns the PostWrite hook) so the os.OpenFile direct call is no longer a leaky abstraction — it''s intentional.'
status: addressed
---
