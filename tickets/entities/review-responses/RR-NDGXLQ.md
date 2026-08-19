---
id: RR-NDGXLQ
type: review-response
title: loadDataEntryPermissions bypasses the injected config.Loader and duplicates an existing helper
finding: 'internal/cli/acl.go:280 reads the config with os.ReadFile(filepath.Join(root, ...)), bypassing the config.Loader that readServices already carries (cli_wiring.go:40, wired as config.NewFSLoader(cfg.FS, cfg.Paths.Root) — rooted at the same directory). config.Loader''s package doc calls itself ''the swap boundary: FSLoader is the default backend; remote or embedded deployments plug in by implementing Loader.'' Reading through os hardcodes the OS filesystem and un-swaps that boundary for this one call, also bypassing storage.FS. Consequence is not cosmetic: on any deployment serving config from a non-OS storage.FS the read fails, which routes into the err != nil branch, which SUPPRESSES A7 entirely with only a warning — the audit silently degrades on a backend where the file is perfectly readable via the injected loader. Worse, internal/cli/analyze.go:679 already has loadDataEntryConfig(svc *readServices) doing exactly this correctly via svc.Config.Load, so this is duplicated logic that diverges from the established pattern in the same package. I checked for an existing helper before writing the adapter but only searched dataentryconfig for a loader, not internal/cli for a caller.'
severity: minor
status: open
---

## Fix

Take `config.Loader` as the parameter and call `cfg.Load(ctx,
dataentryconfig.ConfigFile)`. `FSLoader.Load` already returns an
`os.IsNotExist`-compatible error (pinned by
`TestFSLoader_Load_MissingFileIsNotExist`), so the `stderrors.Is(err,
os.ErrNotExist)` branch keeps working unchanged.

Severity is minor rather than significant: on the default build the behaviour is
identical, and the degradation is fail-safe (suppress A7, warn) rather than a
wrong answer.
