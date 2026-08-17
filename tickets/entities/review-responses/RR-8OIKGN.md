---
id: RR-8OIKGN
type: review-response
title: Metamodel reload doesn't re-reconcile or re-publish unique specs (boot-only)
finding: 'reconcileDerivedSchemaIfSupported + SetUniqueSpecProvider are called once in appbuild.assemble with boot-time base.meta. On a live metamodel reload (data-entry watcher builds a fresh *Metamodel), nothing re-invokes them: a newly-added unique:true gets no index until restart, and currentUniqueSpecs stays stale so a peer-created index''s violation can''t be attributed to a property. The docstrings on SetUniqueSpecProvider (''again on a metamodel reload'') and uniqueSpecsFromMetamodel (''reflected on the next call'') describe a re-invocation that does not exist. FIX: either wire a reload hook (re-publish specs + re-reconcile on watcher reload) or correct the docstrings to state reconcile is boot-only.'
severity: significant
resolution: 'Chose the documented-boot-only option (wiring a reload-triggered re-reconcile is a separate design — DDL off a file watcher needs its own debounce/lock policy). Corrected the misleading docstrings: Store.Reconcile now has a BOOT-ONLY note explaining a live reload does not re-run it and the operator applies a new unique via `rela db reconcile`; SetUniqueSpecProvider and uniqueSpecsFromMetamodel docstrings no longer imply reload re-invocation. The operator workflow (dry-run before deploy, reconcile to apply) is documented in the postgres-backend guide. Re-reconcile-on-reload noted as a future extension.'
status: addressed
---
