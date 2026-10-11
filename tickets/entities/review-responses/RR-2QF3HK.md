---
id: RR-2QF3HK
type: review-response
title: JWT rewordings dropped verified and the code's own name
finding: JWT is a standard, not a vendor. mcp/server.go lost verified-JWT gate; also mcp/principal_test.go, caldavalias, tenant/config.go.
severity: significant
resolution: Reverted all JWT rewordings.
status: addressed
---
