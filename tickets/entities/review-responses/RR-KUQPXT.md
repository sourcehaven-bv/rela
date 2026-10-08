---
id: RR-KUQPXT
type: review-response
title: 'Nits: comments, log attrs, docs accuracy, duplicates'
finding: clamd_integration_test.go names Homebrew as the reason for RELA_TEST_CLAMD_BINDS although the suite skips on darwin, and the separator changed from comma to colon; warnings log env=RELA_SANDBOX_READ_PATHS even when the flag supplied the value; rela-server says 'does not exist' for any Stat error; Spec.ExtraReadOnly comment still calls clamd the motivating case; WithExtraReadOnly has no production caller; EnvSandboxReadPaths godoc says every composition root applies it but cmd/rela-docs does not; transforms.md refuse list omits paths under /proc and /dev and 'keep that behaviour exactly' omits the clamd paths; the empty-list warning repeats per tenant; duplicates are bound twice.
severity: nit
resolution: clamd test comment names a Fedora layout and the comma-to-colon change; warnings log setting='RELA_SANDBOX_READ_PATHS / --sandbox-read-paths'; rela-server distinguishes fs.ErrNotExist from other Stat errors; Spec.ExtraReadOnly comment rewritten; WithExtraReadOnly godoc says why it stays without a production caller; EnvSandboxReadPaths godoc narrowed to binaries that run commands (rela-docs runs none); transforms.md refuse list completed and 'exactly' dropped; the empty-list warning is sync.Once; SetHostReadOnly drops duplicates (TestSetHostReadOnlyDropsDuplicates).
status: addressed
---
