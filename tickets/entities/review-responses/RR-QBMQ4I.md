---
id: RR-QBMQ4I
type: review-response
title: Module source excerpts missing on several surfaces
finding: Inline validations use a single-file inlineSourceFS, lua_eval passes a nil sourceFS, inline automations get no SourceFS, and file-backed automations run via RunString with chunk <string>, so lib/ frames get no excerpt. The plan listed only executor.go and action.go.
severity: significant
resolution: 'Inline automation (wrapScriptError), lua_eval and inline validation rules (inlineFS falls back to project files) read the excerpt from lib/ when the failure is in a module; file automations now run under a real chunk name. Tests per surface: action, automation file+inline, document (internal/script), validation inline (internal/validation), lua_run+lua_eval (internal/mcp).'
status: addressed
---
