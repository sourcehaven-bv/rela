---
id: RR-MEWZO5
type: review-response
title: Legacy // +build syntax ignored, hiding rotted legacy-only files
finding: scripts/check-tagged-tests.sh read only //go:build lines. Go still honours the legacy `// +build` syntax when no //go:build line is present, so a legacy-only tagged file is genuinely excluded from the default build AND was invisible to the guard — another silent pass. Found during self-review before the code review returned.
severity: critical
resolution: 'Discovery now reads both syntaxes and applies the go command''s precedence rule (//go:build wins outright when both are present). Cases: "legacy // +build syntax discovered", "legacy comma syntax drops negated term", "rotted legacy // +build file FAILS", and "//go:build wins over // +build".'
status: addressed
---
