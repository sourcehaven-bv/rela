---
id: RR-AV6RF
type: review-response
title: NewReader must not register mutation bindings at all
finding: The plan's read-only enforcement only pays off if mutation bindings (rela.create_entity, update_entity, delete_entity, create_relation, delete_relation) are NOT registered on a NewReader runtime. If they are registered but return a Go RaiseError("entity manager not available"), we've just renamed the svc.Manager==nil hack. The reader's Lua global table should not contain mutation functions — calling rela.create_entity from a validation script should produce a Lua "attempt to call a nil value" error, not a runtime-time manager-not-available error.
severity: significant
resolution: Accepted. NewReader registers only read bindings (get_entity, list_entities, get_relations, trace_from, trace_to, find_path, search, schema introspection, output, write_file, refresh). Mutation bindings (create_entity, update_entity, delete_entity, create_relation, delete_relation) are absent from the reader's rela.* table entirely. A Lua script calling rela.create_entity on a reader runtime gets 'attempt to call a nil value' from the Lua VM, not a Go RaiseError.
status: addressed
---
