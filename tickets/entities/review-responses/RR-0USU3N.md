---
id: RR-0USU3N
type: review-response
title: db reconcile loads metamodel from CWD, can diverge from server schema
finding: 'loadUniqueSpecs discovers the project from os.Getwd() and loads schema.yaml independently of the metamodel the servers reconciled against (postgres deployments keep the metamodel per-node on disk). Running db reconcile from a checkout with a different schema.yaml converges the SHARED db to that copy — dropping indexes the server relies on or creating unknown ones. Since reconcile mutates the catalog, this is a foot-gun. FIX (min): print the resolved paths.SchemaPath so the operator can confirm; document that reconcile must run against the same schema the servers use.'
severity: minor
resolution: 'runDBReconcile now prints `Reconciling derived schema from <schemaPath>` before converging (loadUniqueSpecs returns the resolved paths.SchemaPath), so an operator running from the wrong checkout sees exactly which schema is driving the mutation. The postgres-backend guide already documents that the metamodel lives per-node on disk. Verified e2e: output shows the resolved path.'
status: addressed
---
