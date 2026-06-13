---
id: RR-QGZG72
type: review-response
title: pgstore empty-text SQL path only covered by the DB-gated conformance run
finding: Production callers can never reach Text=="" in SearchVisible (handleV1Search early-returns, scopeFromParam requires q, executeQuery bails on empty). The ORDER BY e.id path ships unexercised in any CI lane without RELA_TEST_DATABASE_URL.
severity: minor
resolution: TestBuildVisibleSearchSQL (no-DB unit tests in the pgstore package) now exercises the SQL builder including structural shape on every CI lane; the empty-text execution path remains covered by the DB-gated EmptyTextListsVisible conformance case, which the CI postgres job runs. Combination accepted as sufficient.
status: addressed
---
