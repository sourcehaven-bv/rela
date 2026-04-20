---
id: RR-HK9G8
type: review-response
title: FSStore.fs is typed storage.FS; compile-time no-raw-byte-IO is aspirational
finding: interfaces.go defines narrow DirFS omitting ReadFile/WriteFile and docs on fsstore.go:108 warn 'MUST NOT be used for data-file I/O.' But FSStore.fs is declared storage.FS (wide) — not DirFS. Future contributor could add s.fs.WriteFile(...) and it compiles cleanly. AC#3's compiler-enforcement claim is not backed by the actual field type. var _ DirFS = storage.FS(nil) asserts satisfaction, not restriction.
severity: significant
resolution: 'FSStore split into two narrow handles: s.dirs DirFS (no ReadFile/WriteFile/Open) and s.rawReader RawReader (single-method ReadFile only, consumed ONLY by the watcher). FSStore no longer has a storage.FS typed field. Adding s.dirs.ReadFile is now a compile-time type error — AC#3''s compiler-enforcement is finally real. Integration via dirReader adapter in crypto_verify.go bundles the two for integrity.Verify''s FSReader interface.'
status: addressed
---
