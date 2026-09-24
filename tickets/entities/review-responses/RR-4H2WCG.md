---
id: RR-4H2WCG
type: review-response
title: Changed index expression is never rebuilt
finding: Index names hash the spec only; a changed expression keeps an unused index reported as enforced.
severity: significant
resolution: Reconcile compares the desired DDL with sqlite_schema.sql and recreates on mismatch; a test changes the expression and asserts the rebuild.
status: addressed
---
