---
id: RR-UFTCYD6
type: review-response
title: VerifiedFrom needs a documented exception for rela test
finding: principal/principal.go:138-146 limits VerifiedFrom to verified assertions or rela-minted identities.
severity: nit
resolution: Add principal.ToolRelaTest and list it in the VerifiedFrom godoc exceptions; t.as models a CLI/scheduler principal (UnmatchedVerified never set).
status: addressed
---
