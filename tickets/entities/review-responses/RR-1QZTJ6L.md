---
id: RR-1QZTJ6L
type: review-response
title: pg leaf wrong on keyset page 2+
finding: pageAfter added the resume id to closures rooted at ancestors and siblings, so inheriting gates lost parents from page 2 on.
severity: significant
resolution: rollupLeaf saves, clears and restores pageAfter; test pages with after set and an inheriting gate.
status: addressed
---
