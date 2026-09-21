---
id: RR-IKIYM1
type: review-response
title: State-machine prefill is silently overridden, not rejected; applyCreateLock is dry-run-only
finding: 'applyCreateLock (affordances.go:1136-1190) pins every machine-typed property to its entry value, marks it read-only and drops it from _transitions — ''a create is an ENTRY, not a transition''. It is called from exactly one site, write_handler.go:599, the create DRY-RUN. So a duplicated status is neither rejected nor accepted: it is silently replaced by the entry value, with no 4xx telling the client its prefill was discarded. The user watches their prefilled value change with no explanation. This compounds RR-DDY9LG: adoptLockedFieldValues (stagedEntity.ts:18-44) overwrites formData from the dry-run echo, and the commit filter then omits writable===false keys anyway, so the prefill is discarded twice by two independent mechanisms. Note the existing clone endpoint has the same defect: write_handler.go:1201-1213 passes the whole source property map to CreateEntity with no machine-field filtering.'
severity: significant
resolution: Machine-typed properties excluded from the prefill so nothing visible is silently replaced; recorded under "State-machine properties are never carried" with AC10b.
status: addressed
---

## Resolution

The duplicate prefill excludes machine-typed properties outright and lets the
create path supply the entry value, so nothing visible to the user is silently
replaced. A machine field hidden by field-visibility policy stays stripped
(RR-C3OJ33) and is not re-added.

The re-baselining of `originalData` must not fight the in-place mutation
documented at `DynamicForm.vue:1113-1117`.

Recorded on the ticket under "State-machine properties are never carried", with
AC10b.
