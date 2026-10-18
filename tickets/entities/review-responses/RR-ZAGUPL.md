---
id: RR-ZAGUPL
type: review-response
title: Remote MCP channel service may keep a stale realtime.yaml holder after a Configure save
finding: RemoteMCPDeps is fixed at startup (section 3.1), while dataentry watches whichever holder it was last wired with. If Configure reassembles with a new SharedBase, MCP keeps the old holder that nothing reloads. Risk 6 mentions only the ACL evaluator; check and document the config holder as well.
severity: minor
resolution: The remote channel service resolves the current config holder per call through the appbuild accessor; covered by a Configure-save test.
status: addressed
---
