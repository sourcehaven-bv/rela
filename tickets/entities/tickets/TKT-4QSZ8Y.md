---
id: TKT-4QSZ8Y
type: ticket
title: Wire read-side ACL into the MCP server
kind: enhancement
priority: critical
effort: l
status: ready
---

Security gate for shipping content states — design doc §12.2.

MCP is wired allow-all on purpose: `appbuild.WithACL(acl.NopACL{})` at
`internal/cli/mcp_wiring.go:43`; zero `visibility.`/acl references anywhere in
`internal/mcp/`. A draft/published split shipped in this state protects drafts
in the SPA while leaking them over the network-facing MCP surface.

Inject the visibility wrappers (row gate + field redaction, DEC-ZBI39P pattern)
at the MCP wiring site. CLI/local stays a defensible operator trust boundary;
MCP is not. Blocks *shipping* pointers, not starting them.
