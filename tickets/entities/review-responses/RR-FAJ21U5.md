---
id: RR-FAJ21U5
type: review-response
title: An orphan run header cannot be cleaned up
finding: A crash between the header write and the run create left a header that looked running forever, and cancel refused it because no run was stored.
severity: significant
resolution: Engine.CloseHeader closes a header with no stored run. FlowControl.Cancel uses it for a non-terminal header older than one minute, on the same permissions, found through _carries.
status: addressed
---
