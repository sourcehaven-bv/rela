---
id: RR-K2ELC7
type: review-response
title: Process-global sync.Once suppresses the warning for later projects in the desktop app
finding: 'WarnIfLegacySchema uses a package-level sync.Once, correct for the CLI and servers (one project per process) but wrong for rela-desktop, which calls discoverProject per LoadProject and is long-lived. Open a legacy project (warned), switch to a modern one, switch to a DIFFERENT legacy project — silence. The operator concludes the second project is fine. The dedup intent is ''do not spam per request for the same project'', not ''warn once ever'', so the Once should be keyed per project root. Sharper secondary point: desktop writes to os.Stderr, which nobody reads in a packaged .app bundle, so the deprecation notice — the entire backward-compat exit strategy — is effectively unobservable for desktop-only operators; reaching them properly needs UI surfacing (deferred, tracked here). Also: resetWarnOnce() and captureStderr mutate package-global and process-global state respectively, so warn_test.go is not t.Parallel()-safe and carries no comment saying so.'
severity: significant
resolution: Replaced the process-wide sync.Once with a sync.Map keyed on project root, so the notice appears once PER PROJECT. The warning now also names the root, which is what makes two warnings distinguishable. Pinned by a new subtest asserting two distinct legacy projects produce two warnings naming their respective roots — it fails against the old Once. Added the missing 'nothing here may use t.Parallel()' note to the test helper. The desktop-stderr-invisibility point is real but is UI work beyond this ticket's scope; recorded as a deferred follow-up rather than silently dropped.
status: addressed
---
