---
id: RR-0USU3N
type: review-response
title: db reconcile loads metamodel from CWD, can diverge from server schema
finding: 'loadUniqueSpecs discovers the project from os.Getwd() and loads schema.yaml independently of the metamodel the servers reconciled against (postgres deployments keep the metamodel per-node on disk). Running db reconcile from a checkout with a different schema.yaml converges the SHARED db to that copy — dropping indexes the server relies on or creating unknown ones. Since reconcile mutates the catalog, this is a foot-gun. FIX (min): print the resolved paths.SchemaPath so the operator can confirm; document that reconcile must run against the same schema the servers use.'
severity: minor
status: open
---
