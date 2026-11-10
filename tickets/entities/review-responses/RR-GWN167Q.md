---
id: RR-GWN167Q
type: review-response
title: Pull and reconcile diff read remote_credential secrets differently
finding: The pull read through secrets.Load(relaDir, empty) which applied an overrides entry keyed by the empty string; reconcile diff used secrets.Global which ignores overrides.
severity: minor
resolution: secrets.resolve returns only the global section when no script is named. TestGlobal_IgnoresOverrides checks Load with no script too.
status: addressed
---
