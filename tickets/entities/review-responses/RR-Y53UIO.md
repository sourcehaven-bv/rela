---
id: RR-Y53UIO
type: review-response
title: Stale jsString comment + missing carve-out/Host unit tests
finding: '(1) apps_sdk.go:106-111 jsString comment says ''Used only for the fixed appHandshakeType constant'' but it''s used for appHelloType/appThemeType/appHandshakeType — update so nobody trusts the ''only fixed constant'' claim and feeds it user input. (2) Missing focused tests the reviewer flagged: isSensitivePath carve-out boundary (/api/v1/_appsX must NOT be exempt; /api/v1/_apps/ must be), and TestAppBaseURL only feeds clean hosts (covered by the C1 fix''s test). (3) Confirmed clean by review (no action): handshake port-leak closed (source-pinned + once-only), _-prefix shadow guard correct + tested, traversal airtight (os.OpenRoot), appsToV1 leaks no file paths, bridge allow-list Go/TS in sync.'
severity: nit
resolution: 'Fixed the stale jsString comment (now lists all three message-type constants). Added TestIsSensitivePath_AppsCarveOut pinning the carve-out boundary: /api/v1/_apps/<id>/... is exempt while /api/v1/_appsX, bare /api/v1/_apps, /api/v1/tickets, /api/v1/_search stay same-origin gated. Host-validation test added (see RR-MN9ZTB).'
status: addressed
---
