---
id: RR-94SGHA
type: review-response
title: Capabilities.SweepNow is a func in a struct of flags
finding: An optional interface asserted on the store would avoid threading the driver through each conformance test.
severity: nit
reason: Storetest capabilities are declared rather than sniffed (see the Versioning doc); a type assertion would silently skip a backend that forgot the hook. The declared field plus require.NotNil makes the omission fail.
status: wont-fix
---
