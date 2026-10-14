---
id: RR-9S78TV
type: review-response
title: lib in configDirs can break rela db load
finding: CollectProjectConfig walks every configDir, bakes in every file and refuses any symlink. A project with an unrelated top-level lib/ (vendored code, binaries) would fail rela db load or ship that directory in rela.db.
severity: minor
resolution: collectConfigDir stores only *.lua under lib/; lib/README.md in the sqlite test is not stored. Risk section updated.
status: addressed
---
