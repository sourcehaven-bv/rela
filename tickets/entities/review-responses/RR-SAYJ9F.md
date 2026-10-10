---
id: RR-SAYJ9F
type: review-response
title: Reassembly keeps the old owner gate
finding: internal/appbuild/piles.go reuses base.piles on reassembly, so OwnerExists/PersonType from the first acl.Declarative survive a policy reload. Rebuild the Service per assembly and share only the Store.
severity: nit
resolution: SharedBase and Services carry only the pile Store; each assembly builds a fresh Service with the current ACL. TestPiles_ReassemblySharesStoreNotService.
status: addressed
---
