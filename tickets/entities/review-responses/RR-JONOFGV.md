---
id: RR-JONOFGV
type: review-response
title: Docs claimed field rules restrict rela.update_entity in an action
finding: Data-entry actions write through the manager handle that skips the field gate; the docs said acl.yaml field rules restrict the ordinary update and that elevation skips them.
severity: significant
resolution: Docs and godoc now say elevation skips only the row-level update grant and that field write rules never applied to action scripts. The action manager's gating is unchanged.
status: addressed
---
