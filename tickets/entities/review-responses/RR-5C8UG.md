---
id: RR-5C8UG
type: review-response
title: 'write_file on reader: filesystem side effect undermines ''read-only'' framing'
finding: 'internal/lua/runtime.go:407 — rela.write_file is registered inside registerReadBindings. A validation rule (reader) can still write arbitrary files under output/. The refactor''s framing is ''mutation bindings not registered at all'', which a reader could misread as ''runtime cannot cause side effects''. It can — to the filesystem. Fix: move write_file into registerWriteBindings (validation has no reason to write files), or document the capability explicitly on ReadDeps.'
severity: significant
resolution: Moved rela.write_file from registerReadBindings to registerWriteBindings (runtime.go). Reader runtimes now have no filesystem write capability; ReadDeps docstring updated to reflect read-only-everywhere semantics. TestReaderRuntime_MutationBindingsAbsent and TestWriterRuntime_MutationBindingsPresent updated to include write_file in the mutator list — both tests pass.
status: addressed
---
