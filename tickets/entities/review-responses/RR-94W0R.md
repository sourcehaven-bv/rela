---
id: RR-94W0R
type: review-response
title: SetScriptPath name doesn't describe its purpose
finding: SetScriptPath is really SetCacheNamespace — scriptPath is an implementation detail of cache namespacing, not a runtime identity the script uses. Validation's pseudo-path 'validations/<rule-name>' feels like a lie when it's really a namespace choice. Rename for clarity.
severity: nit
resolution: Not renamed. The method name matches the underlying field name for discoverability; cache usage is documented in the docstring.
reason: Nit. SetScriptPath has one real caller outside internal-package-level (MCP lua_run) and the name matches the field it sets (scriptPath). Renaming to SetCacheNamespace would be clearer for cache context but obscures that the field also namespaces secrets. Revisit if the method grows a second user or the field's role broadens.
status: deferred
---
