---
id: RR-52FAV0
type: review-response
title: FSFactory.OpenStore wrapper is a no-op indirection
finding: 'internal/app/factory.go:8-14. OpenStore calls open and re-wraps the result only to convert *FSStore to store.Store. ''return f.open(meta)'' does not compile because of the concrete return type, so either keep it, or let OpenStore call OpenStoreWithSchemaRefresh and drop the refresh. More useful: the nil-meta error in open still says ''FSFactory.OpenStore'' when it comes from OpenStoreWithSchemaRefresh. Make the message name the method actually called, or use a neutral ''app: FSFactory requires a non-nil metamodel''.'
severity: nit
resolution: The nil-metamodel error names FSFactory instead of OpenStore.
status: addressed
---
