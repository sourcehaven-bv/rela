---
id: RR-J2JPHIR
type: review-response
title: rela.write_file writes output/ under the project root
finding: lua/output.go:124-127 joins projectRoot and outputDir.
severity: significant
resolution: Test world points the output dir at a temp dir outside the root (removed after the case); AC6 includes a WriteFile script.
status: addressed
---
