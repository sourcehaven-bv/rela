---
id: RR-9ZB7ZO
type: review-response
title: Stale test reference + misleading bench recipe comment
finding: BenchmarkValidateCreate's comment referenced a nonexistent test (the real pin is TestValidateCreate_SkipsIDGeneration, RR-8I07); the just bench comment promised pgstore benchmarks the recipe doesn't run, and called the markdown-parse benchmark a 'lua' one.
severity: minor
resolution: Comment now points at the real test; recipe comment names the markdown-parse benchmark and documents the postgres-tagged command for the DB-gated pgstore benchmark (kept out of the default recipe — must not link pgx). Also simplified the _, _ = fv, rv nit.
status: addressed
---
