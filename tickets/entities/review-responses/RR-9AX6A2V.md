---
id: RR-9AX6A2V
type: review-response
title: MaxEventBranches and flowcfg.MaxBranches can drift
finding: A kept change stores a branch index capped by flowrun.MaxEventBranches, while the config caps branches with flowcfg.MaxBranches; a raise of the latter makes valid kept events fail CheckUpdate.
severity: significant
resolution: TestEventBranchCap asserts MaxEventBranches >= flowcfg.MaxBranches.
status: addressed
---
