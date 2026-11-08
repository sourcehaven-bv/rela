---
id: RR-4APOTQ6
type: review-response
title: db reconcile and status took the tenant from --project but the indexes from the cwd
finding: 'db_postgres.go: resolveDSN used tenantMapFor(projectPath) while loadDerivedSpecs used os.Getwd(). Run from another checkout, reconcile would converge the tenant''s schema toward the wrong schema.yaml and drop still-wanted derived indexes.'
severity: significant
resolution: resolveDSN returns the discovered project paths with the DSN; loadDerivedSpecs takes them. TestTenantDB_TargetsTheNamedProject pins it.
status: addressed
---
