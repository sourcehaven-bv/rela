---
id: RR-6VQR9O
type: review-response
title: Remote MCPHost wiring is untested
finding: No test asserted that remote MCP gets the App's writeMu, runner and live schema; a wrong wiring would pass every test.
severity: significant
resolution: TestRemoteMCP_HostSharesUploadPolicy asserts WriteLock == &app.writeMu, the shared runner, and that AttachmentPolicy follows a schema Publish.
status: addressed
---
