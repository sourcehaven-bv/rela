---
id: RR-SLFZ1A9
type: review-response
title: 401 no longer hints at RELA_DATAENTRY_USER
finding: The old 500 said check server logs. The 401 gives no pointer to the env var or --principal-header. TKT-QR1VTP targets the old response.
severity: minor
reason: Keeping the 401 body identical to the JWT gate and writeNoIdentity. The hint belongs in TKT-QR1VTP which should now target this 401.
status: deferred
---
