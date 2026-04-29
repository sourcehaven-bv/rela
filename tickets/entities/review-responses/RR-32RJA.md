---
id: RR-32RJA
type: review-response
title: Searcher can nil-panic on q parameter
finding: 'runFreeTextSearch calls svc.Searcher.Search(...) with no nil-check. The constructor takes searcher search.Searcher without validation. Project rule (CLAUDE.md): Constructors reject nil required fields.'
severity: critical
resolution: 'Added nil-validation in NewApp constructor for meta, st, em, and searcher. Returns error ''dataentry.NewApp: <field> is required'' when any required collaborator is nil.'
status: addressed
---
