---
id: RR-CR-DOUBLEREAD
type: review-response
title: 'customAssetExists read up to 4MB per file per shell request only to discard it'
finding: "`customAssetExists` called `openCustomAsset`, which reads the entire file into memory (up to maxCustomFileBytes = 4 MiB) and then throws the bytes away to return a bool. `selectShell` calls it twice per SPA shell request. So every hard refresh, deep-link, and 404-to-shell fallthrough read and discarded up to 8 MB — on the hot path, in a function whose own comment claimed 'no lock on the hot path'. An operator shipping a 3 MB bundled custom.js (plausible; the docs encourage imports) would make every page load burn 6 MB of allocation."
severity: significant
status: addressed
resolution: 'Rewritten to `os.OpenRoot` + `root.Stat` with an IsDir check — no content read. Added TestCustomAssetExists_MatchesOpen pinning that the cheap check agrees with the authoritative read across present/absent/directory/non-allowlisted/symlink-escape, so the two can never diverge into the confusing half-state where the shell links an asset that then 404s.'
---

Raised by `/code-review` of the TKT-3DBK6I implementation.
