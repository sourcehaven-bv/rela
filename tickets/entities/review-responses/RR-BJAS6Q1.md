---
id: RR-BJAS6Q1
type: review-response
title: Tests depend on the environment
finding: validationModeFor reads RELA_VALIDATION_MODE and .rela/config.yaml; GC intervals come from env.
severity: minor
resolution: The write-path test pins RELA_VALIDATION_MODE with t.Setenv. The fixture project has no .rela/config.yaml. Whether NewInMemory should ignore env and project config is a rela test question for part 4.
status: addressed
---
